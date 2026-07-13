package transport

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/DaanV2/itinerarium/api/application"
	"github.com/DaanV2/itinerarium/api/infrastructure/persistence/models"
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currencies, err := svc.ListCurrencies(r.Context())
		if err != nil {
			writeCatalogServiceError(w, err)

			return
		}

		responses := make([]currencyResponse, len(currencies))
		for i := range currencies {
			responses[i] = toCurrencyResponse(&currencies[i])
		}

		writeJSON(w, http.StatusOK, responses)
	})
}

// CreateCurrencyHandler lets a GM add a currency to the catalog. Must be
// wrapped in RequireAuth.
func CreateCurrencyHandler(svc *application.CatalogService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req createCurrencyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		c, err := svc.CreateCurrency(r.Context(), requesterFrom(r), req.Code, req.Name, req.Ratio)
		if err != nil {
			writeCatalogServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, toCurrencyResponse(c))
	})
}

// ListItemDefinitionsHandler returns the item catalog. Must be wrapped in
// RequireAuth.
func ListItemDefinitionsHandler(svc *application.CatalogService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defs, err := svc.ListItemDefinitions(r.Context())
		if err != nil {
			writeCatalogServiceError(w, err)

			return
		}

		responses := make([]itemDefinitionResponse, len(defs))
		for i := range defs {
			responses[i] = toItemDefinitionResponse(&defs[i])
		}

		writeJSON(w, http.StatusOK, responses)
	})
}

// CreateItemDefinitionHandler lets a GM add an item to the catalog. Must be
// wrapped in RequireAuth.
func CreateItemDefinitionHandler(svc *application.CatalogService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req createItemDefinitionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		d, err := svc.CreateItemDefinition(r.Context(), requesterFrom(r), req.Name, req.Description, req.Category)
		if err != nil {
			writeCatalogServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, toItemDefinitionResponse(d))
	})
}

func writeCatalogServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, application.ErrForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, application.ErrCurrencyExists), errors.Is(err, application.ErrItemDefinitionExists):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, application.ErrInvalidCurrency), errors.Is(err, application.ErrInvalidName),
		errors.Is(err, application.ErrInvalidAmount), errors.Is(err, application.ErrNoAmounts):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, application.ErrUnknownCurrency):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "processing request")
	}
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req convertCurrencyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		result, err := svc.Convert(r.Context(), toCurrencyAmounts(req.Amounts), req.To)
		if err != nil {
			writeCatalogServiceError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, convertCurrencyResponse{
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
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req simplifyCurrencyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")

			return
		}

		breakdown, err := svc.Simplify(r.Context(), toCurrencyAmounts(req.Amounts))
		if err != nil {
			writeCatalogServiceError(w, err)

			return
		}

		responses := make([]simplifiedAmountResponse, len(breakdown))
		for i, a := range breakdown {
			responses[i] = simplifiedAmountResponse{Currency: toCurrencyResponse(&a.Currency), Amount: a.Amount}
		}

		writeJSON(w, http.StatusOK, responses)
	})
}
