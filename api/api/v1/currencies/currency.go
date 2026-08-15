package currenciesv1

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/infrastructure/transport"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
)

type simplifyCurrencyRequest struct {
	Amounts []currencyAmountRequest `json:"amounts"`
}

type simplifiedAmountResponse struct {
	Currency currencyResponse `json:"currency"`
	Amount   int64            `json:"amount"`
}

type convertCurrencyRequest struct {
	Amounts []currencyAmountRequest `json:"amounts"`
	To      string                  `json:"to"`
}

type convertCurrencyResponse struct {
	Currency  currencyResponse `json:"currency"`
	Whole     int64            `json:"whole"`
	Remainder int64            `json:"remainder"`
	BaseValue int64            `json:"base_value"`
}

type currencyAmountRequest struct {
	Currency string `json:"currency"`
	Amount   int64  `json:"amount"`
}

type currencyResponse struct {
	ID    string `json:"id"`
	Code  string `json:"code"`
	Name  string `json:"name"`
	Ratio int64  `json:"ratio"`
}

type createCurrencyRequest struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Ratio int64  `json:"ratio"`
}

func toCurrencyAmounts(reqs []currencyAmountRequest) []application.CurrencyAmount {
	amounts := make([]application.CurrencyAmount, len(reqs))
	for i, a := range reqs {
		amounts[i] = application.CurrencyAmount{Currency: a.Currency, Amount: a.Amount}
	}

	return amounts
}

// Route paths for the currency catalog.
const (
	CurrenciesPath       = "/api/currencies"
	CurrencyConvertPath  = CurrenciesPath + "/convert"
	CurrencySimplifyPath = CurrenciesPath + "/simplify"
)

// CurrencyHandler serves the currency catalog under /api/currencies.
type CurrencyHandler struct {
	catalog *application.CatalogService
}

// NewCurrencyHandler builds the currency-catalog handler.
func NewCurrencyHandler(catalog *application.CatalogService) *CurrencyHandler {
	return &CurrencyHandler{catalog: catalog}
}

// Register wires the currency routes onto r. Each handler must be reached
// through RequireAuth.
func (h *CurrencyHandler) Register(r *transport.Router) {
	r.Handle("GET "+CurrenciesPath, h.List())
	r.Handle("POST "+CurrenciesPath, h.Create())
	r.Handle("POST "+CurrencyConvertPath, h.Convert())
	r.Handle("POST "+CurrencySimplifyPath, h.Simplify())
}

// Simplify adds up one or more currency amounts and returns the fewest-coins
// breakdown across the whole catalog.
func (h *CurrencyHandler) Simplify() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req simplifyCurrencyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		breakdown, err := h.catalog.Simplify(r.Context(), toCurrencyAmounts(req.Amounts))
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		responses := make([]simplifiedAmountResponse, len(breakdown))
		for i := range breakdown {
			responses[i] = simplifiedAmountResponse{
				Currency: toCurrencyResponse(&breakdown[i].Currency), Amount: breakdown[i].Amount,
			}
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}

// Create lets a GM add a currency to the catalog.
func (h *CurrencyHandler) Create() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req createCurrencyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		c, err := h.catalog.CreateCurrency(r.Context(), transport.RequesterFrom(r), req.Code, req.Name, req.Ratio)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toCurrencyResponse(c))
	})
}

// Convert adds up one or more currency amounts and expresses the total in a
// target currency — covering both single-currency conversion ("how much of X is
// Y") and adding amounts across currencies together. Any authenticated user may
// call it, currencies are not secret.
func (h *CurrencyHandler) Convert() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req convertCurrencyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		result, err := h.catalog.Convert(r.Context(), toCurrencyAmounts(req.Amounts), req.To)
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusOK, convertCurrencyResponse{
			Currency:  toCurrencyResponse(&result.Currency),
			Whole:     result.Whole,
			Remainder: result.Remainder,
			BaseValue: result.BaseValue,
		})
	})
}

// List returns the currency catalog.
func (h *CurrencyHandler) List() http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		currencies, err := h.catalog.ListCurrencies(r.Context())
		if err != nil {
			transport.WriteServiceError(w, err)

			return
		}

		responses := make([]currencyResponse, len(currencies))
		for i := range currencies {
			responses[i] = toCurrencyResponse(&currencies[i])
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}

func toCurrencyResponse(c *models.Currency) currencyResponse {
	return currencyResponse{ID: c.ID, Code: c.Code, Name: c.Name, Ratio: c.Ratio}
}
