package zoo

import (
	"math"
	"net/http"
	"strconv"
)

// CreateAnimal handles POST /animals.
func (api *API) CreateAnimal(w http.ResponseWriter, r *http.Request) {
	var req CreateAnimal
	if err := decodeJSON(w, r, &req); err != nil {
		api.rejectBody(w, r, "create animal", err)
		return
	}
	if err := req.validate(); err != nil {
		api.logger.WarnContext(r.Context(), "create animal rejected", "reason", "invalid field", "error", err)
		api.writeErrorResponse(w, http.StatusBadRequest, "Error: "+err.Error())
		return
	}

	// BUG-005: silent truncation. Names longer than 64 bytes are cut to 64
	// bytes without an error, which can also split a multi-byte character.
	name := req.Name
	if len(name) > 64 {
		name = name[:64]
	}

	animal := api.animals.create(Animal{Name: name, Species: req.Species, Age: req.Age})
	api.logger.InfoContext(r.Context(), "animal created",
		"id", animal.ID, "name", animal.Name, "species", animal.Species, "age", animal.Age)

	api.writeResponse(w, http.StatusCreated, animal)
}

// DeleteAnimal handles DELETE /animals/{id}.
func (api *API) DeleteAnimal(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		api.logger.WarnContext(r.Context(), "delete animal rejected", "reason", "non-integer id", "id", r.PathValue("id"))
		api.writeErrorResponse(w, http.StatusBadRequest, "ID must be integer")
		return
	}

	// BUG-001: check-then-act. The lookup and the delete take the lock
	// separately, so two concurrent DELETEs of the same animal can both
	// pass the check and both return 204 instead of 204 then 404.
	if _, ok := api.animals.get(id); !ok {
		api.logger.WarnContext(r.Context(), "delete animal rejected", "reason", "not found", "id", id)
		api.writeErrorResponse(w, http.StatusNotFound, "animal not found")
		return
	}
	api.animals.delete(id)
	api.logger.InfoContext(r.Context(), "animal deleted", "id", id)

	w.WriteHeader(http.StatusNoContent)
}

// FindAnimal handles GET /animals/{id}.
func (api *API) FindAnimal(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		api.writeErrorResponse(w, http.StatusBadRequest, "ID must be integer")
		return
	}

	animal, ok := api.animals.get(id)
	if !ok {
		api.writeErrorResponse(w, http.StatusNotFound, "animal not found")
		return
	}

	api.writeResponse(w, http.StatusOK, animal)
}

// PutAnimal replaces the whole animal: name, species and age are all
// required, exactly as in creation. It never creates; an unknown ID is 404.
func (api *API) PutAnimal(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		api.logger.WarnContext(r.Context(), "put animal rejected", "reason", "non-integer id", "id", r.PathValue("id"))
		api.writeErrorResponse(w, http.StatusBadRequest, "ID must be integer")
		return
	}

	var req CreateAnimal
	if err := decodeJSON(w, r, &req); err != nil {
		api.rejectBody(w, r, "put animal", err, "id", id)
		return
	}
	if err := req.validate(); err != nil {
		api.logger.WarnContext(r.Context(), "put animal rejected", "reason", "invalid field", "id", id, "error", err)
		api.writeErrorResponse(w, http.StatusBadRequest, "Error: "+err.Error())
		return
	}

	animal, ok := api.animals.update(id, func(a *Animal) {
		a.Name, a.Species, a.Age = req.Name, req.Species, req.Age
	})
	if !ok {
		api.logger.WarnContext(r.Context(), "put animal rejected", "reason", "not found", "id", id)
		api.writeErrorResponse(w, http.StatusNotFound, "animal not found")
		return
	}

	api.logger.InfoContext(r.Context(), "animal replaced",
		"id", animal.ID, "name", animal.Name, "species", animal.Species, "age", animal.Age)
	api.writeResponse(w, http.StatusOK, animal)
}

// PatchAnimal updates only the fields present in the body. An empty object
// is rejected, and so is any field that fails the same rules as creation.
func (api *API) PatchAnimal(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		api.logger.WarnContext(r.Context(), "patch animal rejected", "reason", "non-integer id", "id", r.PathValue("id"))
		api.writeErrorResponse(w, http.StatusBadRequest, "ID must be integer")
		return
	}

	var req UpdateAnimal
	if err := decodeJSON(w, r, &req); err != nil {
		api.rejectBody(w, r, "patch animal", err, "id", id)
		return
	}
	if err := req.validate(); err != nil {
		api.logger.WarnContext(r.Context(), "patch animal rejected", "reason", "invalid field", "id", id, "error", err)
		api.writeErrorResponse(w, http.StatusBadRequest, "Error: "+err.Error())
		return
	}

	animal, ok := api.animals.update(id, func(a *Animal) {
		if req.Name != nil {
			a.Name = *req.Name
		}
		if req.Species != nil {
			a.Species = *req.Species
		}
		if req.Age != nil {
			// BUG-002: integer wrap-around. PATCH stores age through an
			// int8, so ages of 128 and above come back negative or wrong.
			// PUT and POST keep the full value.
			a.Age = int(int8(*req.Age)) //nolint:gosec // BUG-002: the overflow is the planted bug
		}
	})
	if !ok {
		api.logger.WarnContext(r.Context(), "patch animal rejected", "reason", "not found", "id", id)
		api.writeErrorResponse(w, http.StatusNotFound, "animal not found")
		return
	}

	api.logger.InfoContext(r.Context(), "animal updated",
		"id", animal.ID, "name", animal.Name, "species", animal.Species, "age", animal.Age)
	api.writeResponse(w, http.StatusOK, animal)
}

// ListAnimals returns animals ordered by ID. The optional query parameters
// start and end select the half-open index range [start, end); start
// defaults to 0 and end to the last animal.
func (api *API) ListAnimals(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	start, err := intParam(q.Get("start"), 0)
	if err != nil || start < 0 {
		api.writeErrorResponse(w, http.StatusBadRequest, "start must be a non-negative integer")
		return
	}

	end, err := intParam(q.Get("end"), math.MaxInt)
	if err != nil || end < start {
		api.writeErrorResponse(w, http.StatusBadRequest, "end must be an integer >= start")
		return
	}

	api.writeResponse(w, http.StatusOK, api.animals.list(start, end))
}

// intParam parses a query value, returning def when it is absent.
func intParam(raw string, def int) (int, error) {
	if raw == "" {
		return def, nil
	}
	return strconv.Atoi(raw)
}
