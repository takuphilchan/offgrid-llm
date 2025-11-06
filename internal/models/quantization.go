package models

// QuantizationInfo provides detailed information about quantization methods
type QuantizationInfo struct {
	Name          string
	Description   string
	BitsPerWeight float64
	QualityLevel  string // "Highest", "Very High", "High", "Medium", "Low", "Lowest"
	UseCases      string
}

// GetQuantizationInfo returns detailed information about a quantization method
func GetQuantizationInfo(quantization string) QuantizationInfo {
	infos := map[string]QuantizationInfo{
		"Q2_K": {
			Name:          "Q2_K",
			Description:   "Extremely low quality, smallest size",
			BitsPerWeight: 2.5,
			QualityLevel:  "Lowest",
			UseCases:      "Testing only, not recommended for production",
		},
		"Q3_K_S": {
			Name:          "Q3_K_S",
			Description:   "Very low quality, small size",
			BitsPerWeight: 3.4,
			QualityLevel:  "Low",
			UseCases:      "Severe resource constraints, acceptable for simple tasks",
		},
		"Q3_K_M": {
			Name:          "Q3_K_M",
			Description:   "Low quality, balanced size",
			BitsPerWeight: 3.9,
			QualityLevel:  "Low-Medium",
			UseCases:      "Resource-constrained environments, general tasks",
		},
		"Q3_K_L": {
			Name:          "Q3_K_L",
			Description:   "Moderate quality, larger 3-bit",
			BitsPerWeight: 4.3,
			QualityLevel:  "Medium",
			UseCases:      "Better quality in constrained environments",
		},
		"Q4_0": {
			Name:          "Q4_0",
			Description:   "Older 4-bit quantization, good baseline",
			BitsPerWeight: 4.5,
			QualityLevel:  "Medium",
			UseCases:      "Legacy support, decent quality-to-size ratio",
		},
		"Q4_K_S": {
			Name:          "Q4_K_S",
			Description:   "Newer 4-bit, small variant",
			BitsPerWeight: 4.6,
			QualityLevel:  "Medium-High",
			UseCases:      "Good balance for most use cases",
		},
		"Q4_K_M": {
			Name:          "Q4_K_M",
			Description:   "Recommended 4-bit, best quality-size balance",
			BitsPerWeight: 4.8,
			QualityLevel:  "High",
			UseCases:      "🌟 Recommended for most users - excellent balance",
		},
		"Q5_0": {
			Name:          "Q5_0",
			Description:   "Older 5-bit quantization",
			BitsPerWeight: 5.5,
			QualityLevel:  "High",
			UseCases:      "Higher quality, moderate file size increase",
		},
		"Q5_K_S": {
			Name:          "Q5_K_S",
			Description:   "Newer 5-bit, small variant",
			BitsPerWeight: 5.6,
			QualityLevel:  "Very High",
			UseCases:      "Near-original quality with moderate size",
		},
		"Q5_K_M": {
			Name:          "Q5_K_M",
			Description:   "Recommended 5-bit, excellent quality",
			BitsPerWeight: 5.8,
			QualityLevel:  "Very High",
			UseCases:      "Best quality for production, worth the extra size",
		},
		"Q6_K": {
			Name:          "Q6_K",
			Description:   "Very high quality, approaching FP16",
			BitsPerWeight: 6.6,
			QualityLevel:  "Highest",
			UseCases:      "Maximum quality, use when storage isn't constrained",
		},
		"Q8_0": {
			Name:          "Q8_0",
			Description:   "Extremely high quality, nearly identical to FP16",
			BitsPerWeight: 8.5,
			QualityLevel:  "Highest",
			UseCases:      "Research, benchmarking, when size doesn't matter",
		},
		"F16": {
			Name:          "F16",
			Description:   "16-bit floating point, original quality",
			BitsPerWeight: 16.0,
			QualityLevel:  "Original",
			UseCases:      "Unquantized, requires 2x storage of Q8",
		},
		"F32": {
			Name:          "F32",
			Description:   "32-bit floating point, full precision",
			BitsPerWeight: 32.0,
			QualityLevel:  "Full Precision",
			UseCases:      "Training, rarely needed for inference",
		},
	}

	if info, ok := infos[quantization]; ok {
		return info
	}

	// Default for unknown quantization
	return QuantizationInfo{
		Name:         quantization,
		Description:  "Unknown quantization method",
		QualityLevel: "Unknown",
		UseCases:     "See model documentation",
	}
}

// GetQuantizationRecommendation returns a recommendation based on available RAM
func GetQuantizationRecommendation(availableRAMGB int) string {
	switch {
	case availableRAMGB < 4:
		return "Q3_K_M or Q4_0 - You have limited RAM"
	case availableRAMGB < 8:
		return "Q4_K_M - Best balance for your system"
	case availableRAMGB < 16:
		return "Q4_K_M or Q5_K_M - Your system can handle higher quality"
	case availableRAMGB < 32:
		return "Q5_K_M or Q6_K - Plenty of RAM for high quality"
	default:
		return "Q6_K or Q8_0 - You can use maximum quality"
	}
}
