package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

func checkOPAPolicy(tokenStr string, l *zap.SugaredLogger) (map[string]interface{}, error) {
	ctx := context.Background()
	input := map[string]interface{}{
		"input": map[string]interface{}{
			"token": tokenStr,
		},
	}

	inputData, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	opaURL := "http://opa:8181/v1/data/authz/allow"
	req, err := http.NewRequestWithContext(ctx, "POST", opaURL, bytes.NewBuffer(inputData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("authorization failed: %s", resp.Status)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	allowed, ok := result["result"].(bool)
	if !ok || !allowed {
		return nil, fmt.Errorf("authorization failed")
	}
	l.Info(result)
	return result, nil
}