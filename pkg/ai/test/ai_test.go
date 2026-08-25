package ai_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"invise-backend/pkg/ai"
)

func TestAIClient_Predict_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/predict", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.True(t, strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data"))

		resp := ai.AIPredictResponse{
			Status:        "success",
			ForecastMonth: "2016-06",
			SeriesCount:   1,
			Summary: ai.AISummary{
				TotalForecast: 40.51,
				MeanForecast:  40.51,
			},
			Predictions: []ai.AIPredictionItem{
				{
					ForecastMonth:         "2016-06",
					SeriesID:              "FOODS_1_035_CA_1_evaluation",
					ID:                    "FOODS_1_035_CA_1_evaluation",
					StoreID:               "CA_1",
					ItemID:                "FOODS_1_035",
					CatID:                 "FOODS",
					DeptID:                "FOODS_1",
					StateID:               "CA",
					PredictedMonthlySales: 40.5085,
				},
			},
			FeatureImportance: []ai.AIFeatureImportance{
				{
					Rank:          1,
					Feature:       "monthly_sales",
					DisplayName:   "Riwayat Penjualan Bulanan",
					ImportancePct: 45.2,
					Type:          "encoder",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := ai.NewClient(ts.URL, 5)
	csvData := strings.NewReader("date_month,store_id,item_id,monthly_sales\n2016-05,CA_1,FOODS_1_035,9\n")
	res, err := client.Predict(context.Background(), csvData, "test.csv", true, true, 5)

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, "success", res.Status)
	assert.Equal(t, "2016-06", res.ForecastMonth)
	assert.Equal(t, 1, res.SeriesCount)
	assert.Equal(t, 40.51, res.Summary.TotalForecast)
	assert.Len(t, res.Predictions, 1)
	assert.Equal(t, "FOODS_1_035", res.Predictions[0].ItemID)
	assert.Len(t, res.FeatureImportance, 1)
}

func TestAIClient_Predict_ErrorResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "error",
			"message": "Input dataframe is missing required columns: ['monthly_sales']",
		})
	}))
	defer ts.Close()

	client := ai.NewClient(ts.URL, 5)
	csvData := strings.NewReader("invalid,columns\n1,2\n")
	res, err := client.Predict(context.Background(), csvData, "test.csv", true, false, 5)

	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "missing required columns")
}

func TestAIClient_Health(t *testing.T) {
	t.Run("Healthy", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/v1/health", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(ai.AIHealthResponse{
				Status:         "healthy",
				Service:        "m5-tft-demand-forecasting-api",
				ModelLoaded:    true,
				ArtifactSource: "safetensors",
				Device:         "cpu",
			})
		}))
		defer ts.Close()

		client := ai.NewClient(ts.URL, 5)
		res, err := client.Health(context.Background())
		require.NoError(t, err)
		assert.True(t, res.ModelLoaded)
		assert.Equal(t, "healthy", res.Status)
	})

	t.Run("Unhealthy Server", func(t *testing.T) {
		client := ai.NewClient("http://invalid-unreachable-host:9999", 1)
		res, err := client.Health(context.Background())
		require.Error(t, err)
		assert.Nil(t, res)
	})
}

func TestAIClient_Info(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/info", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ai.AIInfoResponse{
			Status: "success",
			ModelInfo: map[string]any{
				"model_type": "TemporalFusionTransformer",
			},
		})
	}))
	defer ts.Close()

	client := ai.NewClient(ts.URL, 5)
	res, err := client.Info(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "success", res.Status)
	assert.Equal(t, "TemporalFusionTransformer", res.ModelInfo["model_type"])
}
