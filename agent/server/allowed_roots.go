package server

import (
	"encoding/json"
	"errors"
)

func listAllowedRoots() (json.RawMessage, error) {
	if fileFS == nil {
		return nil, errors.New("allowed file roots are not configured")
	}
	roots := make([]fileInfo, 0)
	for _, path := range fileFS.Roots() {
		info, e := describeFile(path, false)
		if e != nil {
			return nil, e
		}
		roots = append(roots, info)
	}
	return json.Marshal(roots)
}
