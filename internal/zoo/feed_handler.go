package zoo

import (
	"errors"
	"net/http"
	"strconv"
	"time"
)

// pathID parses the {id} path value of the request.
func pathID(r *http.Request) (int, error) {
	return strconv.Atoi(r.PathValue("id"))
}

// CreateFeed handles POST /feeds.
func (api *API) CreateFeed(w http.ResponseWriter, r *http.Request) {
	var req CreateFeed
	if err := decodeJSON(w, r, &req); err != nil {
		api.rejectBody(w, r, "create feed", err)
		return
	}
	if err := req.validate(); err != nil {
		api.logger.WarnContext(r.Context(), "create feed rejected", "reason", "invalid field", "error", err)
		api.writeErrorResponse(w, http.StatusBadRequest, "Error: "+err.Error())
		return
	}

	feed := api.feeds.createFeed(Feed{Name: req.Name, Diet: req.Diet, Stock: req.Stock})
	api.logger.InfoContext(r.Context(), "feed created",
		"id", feed.ID, "name", feed.Name, "diet", feed.Diet, "stock", feed.Stock)

	api.writeResponse(w, http.StatusCreated, feed)
}

// ListFeeds handles GET /feeds. The optional start and end query parameters
// select the half-open index range [start, end) over feeds ordered by ID,
// exactly as for animals. The optional filters diet and inStock apply before
// the range.
func (api *API) ListFeeds(w http.ResponseWriter, r *http.Request) {
	start, end, ok := api.rangeParams(w, r)
	if !ok {
		return
	}

	filter, err := feedFilterFrom(r.URL.Query())
	if err != nil {
		api.badQuery(w, err)
		return
	}

	api.writeResponse(w, http.StatusOK, api.feeds.listFeeds(filter, start, end))
}

// FindFeed handles GET /feeds/{id}.
func (api *API) FindFeed(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, "ID must be integer")
		return
	}

	feed, ok := api.feeds.getFeed(id)
	if !ok {
		api.writeErrorResponse(w, http.StatusNotFound, "feed not found")
		return
	}

	api.writeResponse(w, http.StatusOK, feed)
}

// PutFeed replaces the whole feed: name, diet and stock are all required,
// exactly as in creation. It never creates; an unknown ID is 404.
func (api *API) PutFeed(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		api.logger.WarnContext(r.Context(), "put feed rejected", "reason", "non-integer id", "id", r.PathValue("id"))
		api.writeErrorResponse(w, http.StatusBadRequest, "ID must be integer")
		return
	}

	var req CreateFeed
	if err := decodeJSON(w, r, &req); err != nil {
		api.rejectBody(w, r, "put feed", err, "id", id)
		return
	}
	if err := req.validate(); err != nil {
		api.logger.WarnContext(r.Context(), "put feed rejected", "reason", "invalid field", "id", id, "error", err)
		api.writeErrorResponse(w, http.StatusBadRequest, "Error: "+err.Error())
		return
	}

	feed, err := api.feeds.replaceFeed(id, Feed{Name: req.Name, Diet: req.Diet, Stock: req.Stock})
	if err != nil {
		api.logger.WarnContext(r.Context(), "put feed rejected", "reason", "not found", "id", id)
		api.writeErrorResponse(w, http.StatusNotFound, "feed not found")
		return
	}

	api.logger.InfoContext(r.Context(), "feed replaced",
		"id", feed.ID, "name", feed.Name, "diet", feed.Diet, "stock", feed.Stock)
	api.writeResponse(w, http.StatusOK, feed)
}

// RestockFeed handles POST /feeds/{id}/restock: it adds quantity to the stock.
func (api *API) RestockFeed(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		api.logger.WarnContext(r.Context(), "restock feed rejected", "reason", "non-integer id", "id", r.PathValue("id"))
		api.writeErrorResponse(w, http.StatusBadRequest, "ID must be integer")
		return
	}

	var req Restock
	if err := decodeJSON(w, r, &req); err != nil {
		api.rejectBody(w, r, "restock feed", err, "id", id)
		return
	}
	if err := req.validate(); err != nil {
		api.logger.WarnContext(r.Context(), "restock feed rejected", "reason", "invalid field", "id", id, "error", err)
		api.writeErrorResponse(w, http.StatusBadRequest, "Error: "+err.Error())
		return
	}

	feed, err := api.feeds.restock(id, req.Quantity)
	if err != nil {
		api.logger.WarnContext(r.Context(), "restock feed rejected", "reason", "not found", "id", id)
		api.writeErrorResponse(w, http.StatusNotFound, "feed not found")
		return
	}

	api.logger.InfoContext(r.Context(), "feed restocked", "id", feed.ID, "added", req.Quantity, "stock", feed.Stock)
	api.writeResponse(w, http.StatusOK, feed)
}

// DeleteFeed handles DELETE /feeds/{id}. A feed that has been used in a
// feeding cannot be deleted (409).
func (api *API) DeleteFeed(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		api.logger.WarnContext(r.Context(), "delete feed rejected", "reason", "non-integer id", "id", r.PathValue("id"))
		api.writeErrorResponse(w, http.StatusBadRequest, "ID must be integer")
		return
	}

	switch err := api.feeds.deleteFeed(id); {
	case errors.Is(err, errFeedNotFound):
		api.logger.WarnContext(r.Context(), "delete feed rejected", "reason", "not found", "id", id)
		api.writeErrorResponse(w, http.StatusNotFound, "feed not found")
		return
	case errors.Is(err, errFeedInUse):
		api.logger.WarnContext(r.Context(), "delete feed rejected", "reason", "in use", "id", id)
		api.writeErrorResponse(w, http.StatusConflict, "feed is referenced by feedings")
		return
	}

	api.logger.InfoContext(r.Context(), "feed deleted", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

// CreateFeeding handles POST /animals/{id}/feedings. The checks run in this
// order, which is the order the fuzzer has to satisfy:
//
//	400 bad ID or body
//	404 animal missing, then feed missing
//	422 species has no known diet, or the feed does not suit the diet
//	409 not enough stock
func (api *API) CreateFeeding(w http.ResponseWriter, r *http.Request) {
	animalID, err := pathID(r)
	if err != nil {
		api.logger.WarnContext(r.Context(), "create feeding rejected", "reason", "non-integer id", "id", r.PathValue("id"))
		api.writeErrorResponse(w, http.StatusBadRequest, "ID must be integer")
		return
	}

	var req CreateFeeding
	if err := decodeJSON(w, r, &req); err != nil {
		api.rejectBody(w, r, "create feeding", err, "animalId", animalID)
		return
	}
	if err := req.validate(); err != nil {
		api.logger.WarnContext(r.Context(), "create feeding rejected", "reason", "invalid field", "animalId", animalID, "error", err)
		api.writeErrorResponse(w, http.StatusBadRequest, "Error: "+err.Error())
		return
	}

	animal, ok := api.animals.get(animalID)
	if !ok {
		api.logger.WarnContext(r.Context(), "create feeding rejected", "reason", "animal not found", "animalId", animalID)
		api.writeErrorResponse(w, http.StatusNotFound, "animal not found")
		return
	}

	feed, ok := api.feeds.getFeed(req.FeedID) // only for the 404; the diet is checked in the store
	if !ok {
		api.logger.WarnContext(r.Context(), "create feeding rejected", "reason", "feed not found", "animalId", animalID, "feedId", req.FeedID)
		api.writeErrorResponse(w, http.StatusNotFound, "feed not found")
		return
	}

	diet, ok := dietOf(animal.Species)
	if !ok {
		api.logger.WarnContext(r.Context(), "create feeding rejected", "reason", "unknown species", "animalId", animalID, "species", animal.Species)
		api.writeErrorResponse(w, http.StatusUnprocessableEntity, "no diet is known for species "+animal.Species)
		return
	}

	feeding, err := api.feeds.feed(animalID, feed.ID, req.Quantity, diet, time.Now().UTC())
	switch {
	case errors.Is(err, errDietMismatch):
		api.logger.WarnContext(r.Context(), "create feeding rejected", "reason", "diet mismatch",
			"animalId", animalID, "feedId", feed.ID, "animalDiet", diet)
		api.writeErrorResponse(w, http.StatusUnprocessableEntity, "a "+string(diet)+" cannot eat that feed")
		return
	case errors.Is(err, errInsufficientStock):
		api.logger.WarnContext(r.Context(), "create feeding rejected", "reason", "insufficient stock",
			"animalId", animalID, "feedId", feed.ID, "quantity", req.Quantity)
		api.writeErrorResponse(w, http.StatusConflict, "insufficient stock")
		return
	case errors.Is(err, errFeedNotFound):
		// The feed was deleted between the lookup and the feeding.
		api.writeErrorResponse(w, http.StatusNotFound, "feed not found")
		return
	case err != nil:
		api.logger.ErrorContext(r.Context(), "create feeding failed", "error", err)
		api.writeErrorResponse(w, http.StatusInternalServerError, "internal error")
		return
	}

	api.logger.InfoContext(r.Context(), "animal fed",
		"id", feeding.ID, "animalId", animalID, "feedId", feed.ID, "quantity", feeding.Quantity)
	api.writeResponse(w, http.StatusCreated, feeding)
}

// ListFeedings handles GET /animals/{id}/feedings.
func (api *API) ListFeedings(w http.ResponseWriter, r *http.Request) {
	animalID, err := pathID(r)
	if err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, "ID must be integer")
		return
	}
	if _, ok := api.animals.get(animalID); !ok {
		api.writeErrorResponse(w, http.StatusNotFound, "animal not found")
		return
	}

	api.writeResponse(w, http.StatusOK, api.feeds.feedingsOf(animalID))
}
