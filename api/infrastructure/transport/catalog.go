package transport

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
	"github.com/DaanV2/itinerarium/api/pkg/extensions/xhttp"
)

type createCurrencyRequest struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Ratio int64  `json:"ratio"`
}

type currencyResponse struct {
	ID    string `json:"id"`
	Code  string `json:"code"`
	Name  string `json:"name"`
	Ratio int64  `json:"ratio"`
}

func toCurrencyResponse(c *models.Currency) currencyResponse {
	return currencyResponse{ID: c.ID, Code: c.Code, Name: c.Name, Ratio: c.Ratio}
}

type createItemDefinitionRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
}

type itemDefinitionResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
}

func toItemDefinitionResponse(d *models.ItemDefinition) itemDefinitionResponse {
	return itemDefinitionResponse{ID: d.ID, Name: d.Name, Description: d.Description, Category: d.Category}
}

// ListCurrenciesHandler returns the currency catalog. Must be wrapped in
// RequireAuth.
func ListCurrenciesHandler(svc *application.CatalogService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		currencies, err := svc.ListCurrencies(r.Context())
		if err != nil {
			writeServiceError(w, err)

			return
		}

		responses := make([]currencyResponse, len(currencies))
		for i := range currencies {
			responses[i] = toCurrencyResponse(&currencies[i])
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}

// CreateCurrencyHandler lets a GM add a currency to the catalog. Must be
// wrapped in RequireAuth.
func CreateCurrencyHandler(svc *application.CatalogService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req createCurrencyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		c, err := svc.CreateCurrency(r.Context(), requesterFrom(r), req.Code, req.Name, req.Ratio)
		if err != nil {
			writeServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toCurrencyResponse(c))
	})
}

// ListItemDefinitionsHandler returns the item catalog. Must be wrapped in
// RequireAuth.
func ListItemDefinitionsHandler(svc *application.CatalogService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		defs, err := svc.ListItemDefinitions(r.Context())
		if err != nil {
			writeServiceError(w, err)

			return
		}

		responses := make([]itemDefinitionResponse, len(defs))
		for i := range defs {
			responses[i] = toItemDefinitionResponse(&defs[i])
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}

// CreateItemDefinitionHandler lets a GM add an item to the catalog. Must be
// wrapped in RequireAuth.
func CreateItemDefinitionHandler(svc *application.CatalogService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req createItemDefinitionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		d, err := svc.CreateItemDefinition(r.Context(), requesterFrom(r), req.Name, req.Description, req.Category)
		if err != nil {
			writeServiceError(w, err)

			return
		}

		w.WriteJSON(http.StatusCreated, toItemDefinitionResponse(d))
	})
}

type currencyAmountRequest struct {
	Currency string `json:"currency"`
	Amount   int64  `json:"amount"`
}

func toCurrencyAmounts(reqs []currencyAmountRequest) []application.CurrencyAmount {
	amounts := make([]application.CurrencyAmount, len(reqs))
	for i, a := range reqs {
		amounts[i] = application.CurrencyAmount{Currency: a.Currency, Amount: a.Amount}
	}

	return amounts
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

// ConvertCurrencyHandler adds up one or more currency amounts and expresses
// the total in a target currency — covering both single-currency conversion
// ("how much of X is Y") and adding amounts across currencies together. Must
// be wrapped in RequireAuth; any authenticated user may call it, currencies
// are not secret.
func ConvertCurrencyHandler(svc *application.CatalogService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req convertCurrencyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		result, err := svc.Convert(r.Context(), toCurrencyAmounts(req.Amounts), req.To)
		if err != nil {
			writeServiceError(w, err)

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

type simplifyCurrencyRequest struct {
	Amounts []currencyAmountRequest `json:"amounts"`
}

type simplifiedAmountResponse struct {
	Currency currencyResponse `json:"currency"`
	Amount   int64            `json:"amount"`
}

// SimplifyCurrencyHandler adds up one or more currency amounts and returns
// the fewest-coins breakdown across the whole catalog. Must be wrapped in
// RequireAuth.
func SimplifyCurrencyHandler(svc *application.CatalogService) http.Handler {
	return xhttp.JSONHandlerFunc(func(w xhttp.JSONResponseWriter, r *http.Request) {
		var req simplifyCurrencyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteError(http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))

			return
		}

		breakdown, err := svc.Simplify(r.Context(), toCurrencyAmounts(req.Amounts))
		if err != nil {
			writeServiceError(w, err)

			return
		}

		responses := make([]simplifiedAmountResponse, len(breakdown))
		for i := range breakdown {
			responses[i] = simplifiedAmountResponse{Currency: toCurrencyResponse(&breakdown[i].Currency), Amount: breakdown[i].Amount}
		}

		w.WriteJSON(http.StatusOK, responses)
	})
}
