package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/Jidetireni/gender-api/internals/profile/handlers/models"
	"github.com/Jidetireni/gender-api/internals/profile/repository"
	"github.com/biter777/countries"
	"github.com/samber/lo"
)

var nameRegex = regexp.MustCompile(`^[a-zA-Z]+$`)

var (
	ErrUninterpretable = errors.New("Unable to interpret query")
)

func encode[T any](w http.ResponseWriter, status int, v T) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}

	return nil
}

func decode[T any](r *http.Request) (T, error) {
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		return v, fmt.Errorf("decode json: %w", err)
	}
	return v, nil
}

func encodeError(w http.ResponseWriter, err error) {
	var apiErr *models.APIError
	if errors.As(err, &apiErr) {
		statusStr := "error"
		if apiErr.Status == http.StatusBadGateway {
			statusStr = "502"
		}
		encode(w, apiErr.Status, models.ErrorResponse{
			Status:  statusStr,
			Message: apiErr.Message,
		})
		return
	}

	log.Printf("unexpected error: %v", err.Error())
	// Raw / unexpected error — never leak internals to the caller.
	encode(w, http.StatusInternalServerError, models.ErrorResponse{
		Status:  "error",
		Message: "internal server error",
	})
	log.Printf("unexpected error: %v", err)
}

func getQueryParam[T any](query url.Values, key string, converter func(string) (T, error)) (*T, error) {
	val := query.Get(key)
	if val == "" {
		return nil, nil
	}
	converted, err := converter(val)
	if err != nil {
		return nil, err
	}
	return &converted, nil
}

func parseProfileFilters(query url.Values) (*repository.ProfileRepositoryFilter, error) {
	filter := &repository.ProfileRepositoryFilter{}

	gender, err := getQueryParam(query, "gender", func(s string) (string, error) {
		if s != "male" && s != "female" {
			return "", errors.New("must be male or female")
		}
		return s, nil
	})
	if err != nil {
		return nil, fmt.Errorf("gender: %w", err)
	}
	filter.Gender = gender

	countryID, err := getQueryParam(query, "country_id", func(s string) (string, error) {
		upper := strings.ToUpper(s)
		if len(upper) != 2 {
			return "", errors.New("must be a 2-letter ISO code")
		}
		return upper, nil
	})
	if err != nil {
		return nil, fmt.Errorf("country_id: %w", err)
	}
	filter.CountryID = countryID

	ageGroup, err := getQueryParam(query, "age_group", func(s string) (string, error) {
		valid := map[string]struct{}{
			"child": {}, "teenager": {}, "adult": {}, "senior": {},
		}
		if _, ok := valid[s]; !ok {
			return "", errors.New("must be child, teenager, adult or senior")
		}
		return s, nil
	})
	if err != nil {
		return nil, fmt.Errorf("age_group: %w", err)
	}
	filter.AgeGroup = ageGroup

	minAge, err := getQueryParam(query, "min_age", func(s string) (int, error) {
		val, err := strconv.Atoi(s)
		if err != nil || val < 0 {
			return 0, errors.New("must be a positive integer")
		}
		return val, nil
	})
	if err != nil {
		return nil, fmt.Errorf("min_age: %w", err)
	}
	filter.MinAge = minAge

	maxAge, err := getQueryParam(query, "max_age", func(s string) (int, error) {
		val, err := strconv.Atoi(s)
		if err != nil || val < 0 {
			return 0, errors.New("must be a positive integer")
		}
		return val, nil
	})
	if err != nil {
		return nil, fmt.Errorf("max_age: %w", err)
	}
	filter.MaxAge = maxAge

	if filter.MinAge != nil && filter.MaxAge != nil && *filter.MinAge > *filter.MaxAge {
		return nil, errors.New("min_age cannot be greater than max_age")
	}

	minGP, err := getQueryParam(query, "min_gender_probability", func(s string) (float64, error) {
		val, err := strconv.ParseFloat(s, 64)
		if err != nil || val < 0 || val > 1 {
			return 0, errors.New("must be a float between 0 and 1")
		}
		return val, nil
	})
	if err != nil {
		return nil, fmt.Errorf("min_gender_probability: %w", err)
	}
	filter.MinGenderProbability = minGP

	minCP, err := getQueryParam(query, "min_country_probability", func(s string) (float64, error) {
		val, err := strconv.ParseFloat(s, 64)
		if err != nil || val < 0 || val > 1 {
			return 0, errors.New("must be a float between 0 and 1")
		}
		return val, nil
	})
	if err != nil {
		return nil, fmt.Errorf("min_country_probability: %w", err)
	}
	filter.MinCountryProbability = minCP

	return filter, nil
}

func parseQueryOptions(query url.Values) (repository.QueryOptions, error) {
	page, err := getQueryParam(query, "page", func(s string) (int, error) {
		val, err := strconv.Atoi(s)
		if err != nil || val < 1 {
			return 0, errors.New("must be a positive integer")
		}
		return val, nil
	})
	if err != nil {
		return repository.QueryOptions{}, fmt.Errorf("page: %w", err)
	}

	limit, err := getQueryParam(query, "limit", func(s string) (int, error) {
		val, err := strconv.Atoi(s)
		if err != nil || val < 1 {
			return 0, errors.New("must be a positive integer")
		}
		return val, nil
	})
	if err != nil {
		return repository.QueryOptions{}, fmt.Errorf("limit: %w", err)
	}

	return repository.QueryOptions{
		Page:  uint32(lo.FromPtr(page)),
		Limit: uint32(lo.FromPtr(limit)),
	}, nil
}

type normalisedQuery struct {
	raw    string   // "young males from nigeria"
	tokens []string // ["young", "males", "from", "nigeria"]
	set    map[string]bool
}

func normalise(q string) normalisedQuery {
	q = strings.ToLower(q)

	q = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r):
			return r
		case unicode.IsDigit(r):
			return r
		case unicode.IsSpace(r):
			return r
		default:
			return -1
		}
	}, q)

	tokens := strings.Fields(q)
	set := make(map[string]bool, len(tokens))
	for _, token := range tokens {
		set[token] = true
	}

	return normalisedQuery{
		raw:    q,
		tokens: tokens,
		set:    set,
	}
}

func resolveGender(set map[string]bool, filter *repository.ProfileRepositoryFilter) {
	hasMale := set["male"] || set["males"]
	hasFemale := set["female"] || set["females"]

	switch {
	case hasMale && !hasFemale:
		filter.Gender = lo.ToPtr("male")
	case hasFemale && !hasMale:
		filter.Gender = lo.ToPtr("female")
	}
}

func resolveAgeGroup(set map[string]bool, filter *repository.ProfileRepositoryFilter) {
	switch {
	case set["teenager"] || set["teenagers"] || set["teen"]:
		filter.AgeGroup = lo.ToPtr("teenager")
	case set["child"] || set["children"]:
		filter.AgeGroup = lo.ToPtr("child")
	case set["adult"] || set["adults"]:
		filter.AgeGroup = lo.ToPtr("adult")
	case set["senior"] || set["seniors"]:
		filter.AgeGroup = lo.ToPtr("senior")
	}
}

func resolveAgeRange(tokens []string, filter *repository.ProfileRepositoryFilter) {
	for i, token := range tokens {
		isMin := lo.Contains([]string{"above", "over", "older"}, token)
		isMax := lo.Contains([]string{"below", "under", "younger"}, token)

		if !isMin && !isMax {
			continue
		}

		// Look for a number near the trigger (up to 2 positions away)
		var val int
		var found bool

		// Search around the trigger
		indices := []int{i + 1, i + 2, i - 1, i - 2}
		for _, idx := range indices {
			if idx >= 0 && idx < len(tokens) {
				if v, err := strconv.Atoi(tokens[idx]); err == nil {
					val, found = v, true
					break
				}
			}
		}

		if found {
			if isMin {
				filter.MinAge = lo.ToPtr(val)
			} else {
				filter.MaxAge = lo.ToPtr(val)
			}
		}
	}
}

func resolveAge(set map[string]bool, filter *repository.ProfileRepositoryFilter) {
	if set["young"] {
		filter.MinAge = lo.ToPtr(16)
		filter.MaxAge = lo.ToPtr(24)
	}
}

func resolveCountry(raw string, filter *repository.ProfileRepositoryFilter) {
	for _, trigger := range []string{"from ", "in "} {
		_, after, found := strings.Cut(raw, trigger)
		if !found {
			continue
		}
		candidate := strings.TrimSpace(after)
		words := strings.Fields(candidate)

		for size := min(3, len(words)); size >= 1; size-- {
			attempt := strings.Join(words[:size], " ")
			c := countries.ByName(attempt)
			if c != countries.Unknown {
				filter.CountryID = lo.ToPtr(c.Alpha2())
			}
		}
	}
}

func parseSearchQuery(q string) (*repository.ProfileRepositoryFilter, error) {
	nq := normalise(q)
	if nq.raw == "" {
		return nil, ErrUninterpretable
	}

	filter := &repository.ProfileRepositoryFilter{}
	resolveGender(nq.set, filter)
	resolveAgeGroup(nq.set, filter)
	resolveAgeRange(nq.tokens, filter)
	resolveAge(nq.set, filter)
	resolveCountry(nq.raw, filter)

	if isFilterEmpty(filter) {
		return nil, ErrUninterpretable
	}

	return filter, nil
}

func isFilterEmpty(f *repository.ProfileRepositoryFilter) bool {
	return f.ID == nil && f.Gender == nil && f.CountryID == nil &&
		f.AgeGroup == nil && f.MinAge == nil && f.MaxAge == nil &&
		f.MinGenderProbability == nil && f.MinCountryProbability == nil
}
