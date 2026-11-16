package rag

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
)

// Versioned little-endian float32 projection. The original JSON vector remains
// available for older index readers; this derived column can be rebuilt without
// invoking an embedding model or altering index identity.
func encodeVector(values []float32) []byte {
	out := make([]byte, 8+4*len(values))
	copy(out, "OGV1")
	binary.LittleEndian.PutUint32(out[4:], uint32(len(values)))
	for i, value := range values {
		binary.LittleEndian.PutUint32(out[8+4*i:], math.Float32bits(value))
	}
	return out
}

func decodeVector(data []byte) ([]float32, error) {
	if len(data) < 8 || string(data[:4]) != "OGV1" {
		return nil, fmt.Errorf("unsupported vector encoding")
	}
	n := int(binary.LittleEndian.Uint32(data[4:8]))
	if n > 65536 || len(data) != 8+4*n {
		return nil, fmt.Errorf("invalid vector length")
	}
	values := make([]float32, n)
	for i := range values {
		values[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[8+4*i:]))
		if math.IsNaN(float64(values[i])) || math.IsInf(float64(values[i]), 0) {
			return nil, fmt.Errorf("non-finite vector")
		}
	}
	return values, nil
}

func (s *SQLiteStore) initBinaryVectors() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Older writers may update the compatibility JSON column alone. Never rank
	// with a stale projection; use JSON until the next bounded startup backfill.
	if _, err = tx.Exec(`CREATE TRIGGER IF NOT EXISTS embedding_projection_invalidate AFTER UPDATE OF embedding ON chunks WHEN new.embedding IS NOT old.embedding AND new.embedding_v1 IS old.embedding_v1 BEGIN UPDATE chunks SET embedding_v1=NULL WHERE rowid=new.rowid; END`); err != nil {
		return err
	}
	var after int64
	for {
		rows, err := tx.Query(`SELECT rowid,embedding FROM chunks WHERE rowid>? AND embedding_v1 IS NULL ORDER BY rowid LIMIT 128`, after)
		if err != nil {
			return err
		}
		type item struct {
			id   int64
			data []byte
		}
		var batch []item
		for rows.Next() {
			var x item
			if err := rows.Scan(&x.id, &x.data); err != nil {
				rows.Close()
				return err
			}
			batch = append(batch, x)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}
		for _, x := range batch {
			var values []float32
			if len(x.data) > 0 {
				if err = json.Unmarshal(x.data, &values); err != nil {
					return fmt.Errorf("cannot migrate malformed vector: %w", err)
				}
			}
			encoded := encodeVector(values)
			if _, err = decodeVector(encoded); err != nil {
				return err
			}
			if _, err = tx.Exec(`UPDATE chunks SET embedding_v1=? WHERE rowid=?`, encoded, x.id); err != nil {
				return err
			}
			after = x.id
		}
	}
	return tx.Commit()
}
