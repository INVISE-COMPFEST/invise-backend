package response_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"invise-backend/pkg/response"
)

func TestResponse_WithMessageOnly(t *testing.T) {
	resp := dto.Response[any]{
		Message: "operation successful",
	}
	assert.Equal(t, "operation successful", resp.Message)
	assert.Nil(t, resp.Data)

	bytes, err := json.Marshal(resp)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"message":"operation successful"}`, string(bytes))
}

func TestResponse_WithData(t *testing.T) {
	type SampleData struct {
		AccessToken string `json:"access_token"`
	}

	data := SampleData{AccessToken: "sample-token-123"}
	resp := dto.Response[SampleData]{
		Message: "login successful",
		Data:    data,
	}
	assert.Equal(t, "login successful", resp.Message)
	assert.Equal(t, data, resp.Data)

	bytes, err := json.Marshal(resp)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"message":"login successful","data":{"access_token":"sample-token-123"}}`, string(bytes))
}

func TestResponse_WithPointerDataNil(t *testing.T) {
	type SampleData struct {
		AccessToken string `json:"access_token"`
	}

	var data *SampleData
	resp := dto.Response[*SampleData]{
		Message: "no data provided",
		Data:    data,
	}

	bytes, err := json.Marshal(resp)
	assert.NoError(t, err)
	assert.JSONEq(t, `{"message":"no data provided"}`, string(bytes))
}
