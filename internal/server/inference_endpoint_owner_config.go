package server

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
)

func (s *Server) endpointConfigurationPath() string {
	if s == nil || s.config == nil {
		return ""
	}
	if s.config.InferenceEndpointsFile != "" {
		return s.config.InferenceEndpointsFile
	}
	if !filepath.IsAbs(s.config.DataDir) {
		return ""
	}
	return filepath.Join(s.config.DataDir, "inference-endpoints.json")
}

func (s *Server) readEndpointConfiguration() (inferenceEndpointFile, string, error) {
	var configuration inferenceEndpointFile
	path := s.endpointConfigurationPath()
	raw, err := readEndpointOwnerJSON(path, 64<<10, &configuration)
	if errors.Is(err, os.ErrNotExist) && s.config.InferenceEndpointsFile == "" {
		return inferenceEndpointFile{Version: 1, Endpoints: []inferenceEndpointEnrollment{}}, endpointOwnerDigest([]byte("absent-owner-file")), nil
	}
	if err != nil {
		return configuration, "", err
	}
	if err := validateInferenceEndpointFile(configuration); err != nil {
		return configuration, "", err
	}
	return configuration, endpointOwnerDigest(raw), nil
}

func replaceEndpointOwnerEntry(configuration inferenceEndpointFile, replace string, entry inferenceEndpointEnrollment) (inferenceEndpointFile, error) {
	configuration.Endpoints = slices.Clone(configuration.Endpoints)
	index := slices.IndexFunc(configuration.Endpoints, func(candidate inferenceEndpointEnrollment) bool { return endpointKey(candidate) == replace })
	if replace != "" && index < 0 {
		return configuration, errors.New("endpoint being updated is no longer in the owner file")
	}
	if index >= 0 {
		if configuration.Endpoints[index].ID != entry.ID || configuration.Endpoints[index].Revision == entry.Revision {
			return configuration, errors.New("an update preserves the endpoint ID and requires a new revision")
		}
		configuration.Endpoints[index] = entry
	} else {
		configuration.Endpoints = append(configuration.Endpoints, entry)
	}
	if err := validateInferenceEndpointFile(configuration); err != nil {
		return configuration, err
	}
	raw, err := json.Marshal(configuration)
	if err != nil || len(raw) > 64<<10 {
		return configuration, errors.New("endpoint owner configuration exceeds its storage bound")
	}
	return configuration, nil
}
