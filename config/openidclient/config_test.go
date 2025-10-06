package openidclient

import (
	"testing"
)

func TestConnectionDetailsExtraction(t *testing.T) {
	// Create a mock resource with the connection details function
	// This simulates what Configure() sets up
	connectionDetailsFn := func(attr map[string]interface{}) (map[string][]byte, error) {
		conn := map[string][]byte{}

		// Expose client_secret with both legacy and simplified key names
		if clientSecret, ok := attr["client_secret"].(string); ok && clientSecret != "" {
			conn["attribute.client_secret"] = []byte(clientSecret) // Legacy format for backward compatibility
			conn["clientSecret"] = []byte(clientSecret)             // Simplified format
		}

		// Expose client_id
		if clientID, ok := attr["client_id"].(string); ok && clientID != "" {
			conn["clientID"] = []byte(clientID)
			conn["attribute.client_id"] = []byte(clientID) // Legacy format
		}

		// Expose service_account_user_id if available
		if serviceAccountUserID, ok := attr["service_account_user_id"].(string); ok && serviceAccountUserID != "" {
			conn["serviceAccountUserId"] = []byte(serviceAccountUserID)
			conn["attribute.service_account_user_id"] = []byte(serviceAccountUserID) // Legacy format
		}

		return conn, nil
	}

	// Test connection details extraction
	testAttrs := map[string]interface{}{
		"client_secret":           "test-secret-123",
		"client_id":               "test-client",
		"service_account_user_id": "user-123",
	}

	conn, err := connectionDetailsFn(testAttrs)
	if err != nil {
		t.Fatalf("Failed to extract connection details: %v", err)
	}

	// Check clientSecret keys
	if string(conn["clientSecret"]) != "test-secret-123" {
		t.Errorf("Expected clientSecret to be 'test-secret-123', got '%s'", string(conn["clientSecret"]))
	}
	if string(conn["attribute.client_secret"]) != "test-secret-123" {
		t.Errorf("Expected attribute.client_secret to be 'test-secret-123', got '%s'", string(conn["attribute.client_secret"]))
	}

	// Check clientID keys
	if string(conn["clientID"]) != "test-client" {
		t.Errorf("Expected clientID to be 'test-client', got '%s'", string(conn["clientID"]))
	}
	if string(conn["attribute.client_id"]) != "test-client" {
		t.Errorf("Expected attribute.client_id to be 'test-client', got '%s'", string(conn["attribute.client_id"]))
	}

	// Check serviceAccountUserId keys
	if string(conn["serviceAccountUserId"]) != "user-123" {
		t.Errorf("Expected serviceAccountUserId to be 'user-123', got '%s'", string(conn["serviceAccountUserId"]))
	}
	if string(conn["attribute.service_account_user_id"]) != "user-123" {
		t.Errorf("Expected attribute.service_account_user_id to be 'user-123', got '%s'", string(conn["attribute.service_account_user_id"]))
	}

	t.Logf("Connection details extracted successfully: %d keys", len(conn))
}

func TestConnectionDetailsWithEmptyValues(t *testing.T) {
	connectionDetailsFn := func(attr map[string]interface{}) (map[string][]byte, error) {
		conn := map[string][]byte{}

		if clientSecret, ok := attr["client_secret"].(string); ok && clientSecret != "" {
			conn["attribute.client_secret"] = []byte(clientSecret)
			conn["clientSecret"] = []byte(clientSecret)
		}

		if clientID, ok := attr["client_id"].(string); ok && clientID != "" {
			conn["clientID"] = []byte(clientID)
			conn["attribute.client_id"] = []byte(clientID)
		}

		if serviceAccountUserID, ok := attr["service_account_user_id"].(string); ok && serviceAccountUserID != "" {
			conn["serviceAccountUserId"] = []byte(serviceAccountUserID)
			conn["attribute.service_account_user_id"] = []byte(serviceAccountUserID)
		}

		return conn, nil
	}

	// Test with empty client_secret
	testAttrs := map[string]interface{}{
		"client_secret": "",
		"client_id":     "test-client",
	}

	conn, err := connectionDetailsFn(testAttrs)
	if err != nil {
		t.Fatalf("Failed to extract connection details: %v", err)
	}

	// Empty values should not be included
	if _, exists := conn["clientSecret"]; exists {
		t.Error("Empty clientSecret should not be included in connection details")
	}

	// Non-empty values should be included
	if _, exists := conn["clientID"]; !exists {
		t.Error("Non-empty clientID should be included in connection details")
	}
}

func TestConnectionDetailsWithMissingValues(t *testing.T) {
	connectionDetailsFn := func(attr map[string]interface{}) (map[string][]byte, error) {
		conn := map[string][]byte{}

		if clientSecret, ok := attr["client_secret"].(string); ok && clientSecret != "" {
			conn["attribute.client_secret"] = []byte(clientSecret)
			conn["clientSecret"] = []byte(clientSecret)
		}

		if clientID, ok := attr["client_id"].(string); ok && clientID != "" {
			conn["clientID"] = []byte(clientID)
			conn["attribute.client_id"] = []byte(clientID)
		}

		if serviceAccountUserID, ok := attr["service_account_user_id"].(string); ok && serviceAccountUserID != "" {
			conn["serviceAccountUserId"] = []byte(serviceAccountUserID)
			conn["attribute.service_account_user_id"] = []byte(serviceAccountUserID)
		}

		return conn, nil
	}

	// Test with missing fields
	testAttrs := map[string]interface{}{
		"client_id": "test-client",
		// client_secret and service_account_user_id are missing
	}

	conn, err := connectionDetailsFn(testAttrs)
	if err != nil {
		t.Fatalf("Failed to extract connection details: %v", err)
	}

	// Only client_id keys should be present
	if len(conn) != 2 {
		t.Errorf("Expected 2 keys (clientID and attribute.client_id), got %d", len(conn))
	}

	if _, exists := conn["clientID"]; !exists {
		t.Error("clientID should be present")
	}
	if _, exists := conn["attribute.client_id"]; !exists {
		t.Error("attribute.client_id should be present")
	}
}
