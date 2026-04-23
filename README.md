# Profiles API

This is a robust RESTful API built in Go that generates comprehensive demographic profiles based on a person's name. It acts as an orchestration layer, combining data from three external APIs ([Genderize](https://genderize.io/), [Agify](https://agify.io/), and [Nationalize](https://nationalize.io/)) to predict a person's gender, age, age group, and nationality.

To ensure high performance and reliability, the application utilizes **Redis** for caching and **PostgreSQL** for persistent storage.

## Features

- **Data Orchestration**: Concurrently fetches real-time demographic predictions from Genderize, Agify, and Nationalize APIs.
- **Natural Language Search**: Powerful rule-based parser that converts plain English queries into database filters.
- **Smart Categorization**: Automatically categorizes profiles into `age_group`s (`child`, `teenager`, `adult`, `senior`) based on the predicted age.
- **Caching Layer**: Utilizes Redis to cache API responses, drastically reducing latency and external API rate limit consumption.
- **Containerized Setup**: Fully dockerized environment for seamless local development.

## Natural Language Query (Search)

The API includes a core feature that allows users to search for profiles using plain English queries via the `GET /api/profiles/search?q=...` endpoint.

### Supported Keywords & Mappings
The parser uses a deterministic, rule-based approach to extract filters:

| Keyword Category | Examples | Logic / Mapping |
| :--- | :--- | :--- |
| **Gender** | `male`, `males`, `female`, `females` | Sets the `gender` filter. If both are mentioned, the filter is cleared. |
| **Age Keywords** | `young` | Specifically maps to a range of **16–24** years. |
| **Age Groups** | `child`, `teenager`, `adult`, `senior` | Maps directly to the stored `age_group` column. |
| **Age Ranges** | `above 30`, `20 and above`, `younger than 18` | Detects numeric values within a 2-word radius of triggers like `above`, `over`, `under`, `below`. |
| **Location** | `from Nigeria`, `in United States` | Detects country names following `from` or `in`. |

### How the Logic Works
1. **Normalization**: The query is converted to lowercase, stripped of punctuation, and tokenized into a word set and a sequence of tokens.
2. **Gender/Age Group Resolution**: The parser checks for the presence of specific keywords in the word set (e.g., "males" -> gender=male).
3. **Age Range Resolution**: It scans the token sequence for age triggers. When a trigger is found, it looks at the surrounding tokens (up to 2 steps forward or backward) for a numeric value. This allows for flexible phrasing like "above 20" or "20 and above".
4. **Keyword Mapping**: Fixed keywords like "young" are applied to set specific numeric bounds.
5. **Country Detection (Sliding Window)**:
   - The parser looks for the triggers "from" or "in".
   - It performs a **sliding window search** on the words following the trigger (checking 3-word, then 2-word, then 1-word combinations) against a comprehensive ISO-3166 country database.
   - Example: For "from south africa", it will first check "south africa", find a match, and resolve to country code `ZA`.

### Limitations & Edge Cases
- **No Semantic Parsing**: The parser is rule-based and does not understand context. For example, "people who like Nigeria" might be misinterpreted as "people from Nigeria" if the trigger words are present.
- **Ambiguous Genders**: If a query contains both "male" and "female" (e.g., "males and females"), the gender filter is ignored entirely to return results for both.
- **Precedence**: Specific age ranges (e.g., "above 30") can be overwritten if the keyword "young" is also present, as "young" has a strict priority definition (16-24).
- **Triggers**: Locations are only resolved if preceded by the specific triggers "from" or "in". Simply typing "Nigeria" in the search query will not trigger the country filter.
- **Numeric Radius**: Age ranges only work if the number is within 2 words of the trigger word (e.g., "above 20" and "20 and above" work, but "above the age of exactly 20" does not).


## API Documentation

### 1. Create Profile
**Endpoint:** `POST /api/profiles`
**Body:** `{"name": "peter"}`

### 2. Natural Language Search
**Endpoint:** `GET /api/profiles/search`
**Param:** `q` (string)
**Example:** `/api/profiles/search?q=young males from nigeria`

### 3. List Profiles (Standard Filtering)
**Endpoint:** `GET /api/profiles`
**Params:** `gender`, `country_id`, `age_group`, `min_age`, `max_age`

---

## Technical Setup

### Prerequisites
- Docker & Docker Compose
- Go 1.25+

### Running the Application
1. **Environment**: Create a `.env` file based on the example in the repository.
2. **Start Services**:
   ```bash
   docker-compose up --build
   ```
3. **Seed Data** (Optional):
   ```bash
   go run cmd/seed/main.go
   ```
