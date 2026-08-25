package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	pkgerr "invise-backend/pkg/errors"
)

type AIPredictResponse struct {
	Status            string                `json:"status"`
	ForecastMonth     string                `json:"forecast_month"`
	SeriesCount       int                   `json:"series_count"`
	Summary           AISummary             `json:"summary,omitempty"`
	Predictions       []AIPredictionItem    `json:"predictions"`
	FeatureImportance []AIFeatureImportance `json:"feature_importance,omitempty"`
	Message           string                `json:"message,omitempty"`
}

type AISummary struct {
	TotalForecast  float64 `json:"total_forecast"`
	MeanForecast   float64 `json:"mean_forecast"`
	MedianForecast float64 `json:"median_forecast"`
	MinForecast    float64 `json:"min_forecast"`
	MaxForecast    float64 `json:"max_forecast"`
	StdForecast    float64 `json:"std_forecast"`
}

type AIPredictionItem struct {
	ForecastMonth         string  `json:"forecast_month"`
	SeriesID              string  `json:"series_id"`
	ID                    string  `json:"id"`
	StoreID               string  `json:"store_id"`
	ItemID                string  `json:"item_id"`
	CatID                 string  `json:"cat_id"`
	DeptID                string  `json:"dept_id"`
	StateID               string  `json:"state_id"`
	PredictedMonthlySales float64 `json:"predicted_monthly_sales"`
}

type AIFeatureImportance struct {
	Rank          int     `json:"rank"`
	Feature       string  `json:"feature"`
	DisplayName   string  `json:"display_name"`
	ImportancePct float64 `json:"importance_pct"`
	Type          string  `json:"type"`
}

type AIHealthResponse struct {
	Status         string `json:"status"`
	Service        string `json:"service"`
	ModelLoaded    bool   `json:"model_loaded"`
	ArtifactSource string `json:"artifact_source"`
	Device         string `json:"device"`
	TimestampUTC   string `json:"timestamp_utc"`
}

type AIInfoResponse struct {
	Status       string         `json:"status"`
	ModelInfo    map[string]any `json:"model_info"`
	TimestampUTC string         `json:"timestamp_utc"`
}

type AIClientI interface {
	Predict(ctx context.Context, salesCSV io.Reader, filename string, includeSummary, includeFeatureImportance bool, topNFeatures int) (*AIPredictResponse, error)
	Health(ctx context.Context) (*AIHealthResponse, error)
	Info(ctx context.Context) (*AIInfoResponse, error)
}

type aiClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string, timeoutSeconds int) AIClientI {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}
	cleanURL := strings.TrimRight(baseURL, "/")
	return &aiClient{
		baseURL: cleanURL,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
	}
}

func (c *aiClient) Predict(
	ctx context.Context,
	salesCSV io.Reader,
	filename string,
	includeSummary, includeFeatureImportance bool,
	topNFeatures int,
) (*AIPredictResponse, error) {
	if filename == "" {
		filename = "monthly_sales_data.csv"
	}
	if topNFeatures <= 0 {
		topNFeatures = 10
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, pkgerr.InternalServerError("AI_REQUEST_FAILED", "failed to create multipart form file")
	}
	if _, err := io.Copy(part, salesCSV); err != nil {
		return nil, pkgerr.InternalServerError("AI_REQUEST_FAILED", "failed to copy CSV payload")
	}

	_ = writer.WriteField("include_summary", strconv.FormatBool(includeSummary))
	_ = writer.WriteField("include_feature_importance", strconv.FormatBool(includeFeatureImportance))
	_ = writer.WriteField("top_n_features", strconv.Itoa(topNFeatures))

	if err := writer.Close(); err != nil {
		return nil, pkgerr.InternalServerError("AI_REQUEST_FAILED", "failed to close multipart writer")
	}

	url := fmt.Sprintf("%s/api/v1/predict", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, pkgerr.InternalServerError("AI_REQUEST_FAILED", "failed to create AI predict request")
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, pkgerr.BadGateway("AI_SERVICE_UNAVAILABLE", fmt.Sprintf("failed to contact AI service: %s", err.Error()))
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, pkgerr.BadGateway("AI_SERVICE_ERROR", "failed to read AI service response body")
	}

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}
		if jsonErr := json.Unmarshal(respBytes, &errResp); jsonErr == nil && errResp.Message != "" {
			return nil, pkgerr.BadGateway("AI_SERVICE_ERROR", fmt.Sprintf("AI service error: %s", errResp.Message))
		}
		return nil, pkgerr.BadGateway("AI_SERVICE_ERROR", fmt.Sprintf("AI service returned status code %d: %s", resp.StatusCode, string(respBytes)))
	}

	var predictRes AIPredictResponse
	if err := json.Unmarshal(respBytes, &predictRes); err != nil {
		return nil, pkgerr.BadGateway("AI_SERVICE_ERROR", "failed to parse AI service prediction JSON")
	}

	return &predictRes, nil
}

func (c *aiClient) Health(ctx context.Context) (*AIHealthResponse, error) {
	url := fmt.Sprintf("%s/api/v1/health", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, pkgerr.InternalServerError("AI_REQUEST_FAILED", "failed to create AI health request")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, pkgerr.BadGateway("AI_SERVICE_UNAVAILABLE", fmt.Sprintf("AI service is unreachable: %s", err.Error()))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, pkgerr.BadGateway("AI_SERVICE_ERROR", fmt.Sprintf("AI service health check failed with status %d", resp.StatusCode))
	}

	var healthRes AIHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&healthRes); err != nil {
		return nil, pkgerr.BadGateway("AI_SERVICE_ERROR", "failed to parse AI service health JSON")
	}

	return &healthRes, nil
}

func (c *aiClient) Info(ctx context.Context) (*AIInfoResponse, error) {
	url := fmt.Sprintf("%s/api/v1/info", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, pkgerr.InternalServerError("AI_REQUEST_FAILED", "failed to create AI info request")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, pkgerr.BadGateway("AI_SERVICE_UNAVAILABLE", fmt.Sprintf("AI service is unreachable: %s", err.Error()))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, pkgerr.BadGateway("AI_SERVICE_ERROR", fmt.Sprintf("AI service info check failed with status %d", resp.StatusCode))
	}

	var infoRes AIInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&infoRes); err != nil {
		return nil, pkgerr.BadGateway("AI_SERVICE_ERROR", "failed to parse AI service info JSON")
	}

	return &infoRes, nil
}
