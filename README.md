# profiles API (Profile Generator)

This is a robust RESTful API built in Go that generates comprehensive demographic profiles based on a person's name. It acts as an orchestration layer, combining data from three external APIs ([Genderize](https://genderize.io/), [Agify](https://agify.io/), and [Nationalize](https://nationalize.io/)) to predict a person's gender, age, age group, and nationality. 

To ensure high performance and reliability, the application utilizes **Redis** for caching and **PostgreSQL** for persistent storage.

## Features

- **Data Orchestration**: Concurrently fetches real-time demographic predictions from Genderize, Agify, and Nationalize APIs.
- **Smart Categorization**: Automatically categorizes profiles into `age_group`s (`child`, `teenager`, `adult`, `senior`) based on the predicted age.
- **Caching Layer**: Utilizes Redis to cache API responses, drastically reducing latency and external API rate limit consumption for repeated requests.
- **Persistent Storage**: Stores generated profiles in a PostgreSQL database, allowing for complex querying and long-term data retention.
- **Advanced Filtering**: Search and filter saved profiles by gender, country, and age group.
- **Containerized Setup**: Fully dockerized environment with `docker-compose` for zero-friction local development (includes App, Postgres, and Redis).
- **Database Migrations**: Automated schema management using `goose`.

## Tech Stack

- **Language**: Go 1.22+
- **Database**: PostgreSQL (managed via `sqlx` and `squirrel` for query building)
- **Caching**: Redis
- **Router**: `go-chi/chi`
- **Migrations**: `goose`
- **Infrastructure**: Docker & Docker Compose

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose (Recommended)
- [Go](https://golang.org/dl/) 1.22+ (if running natively without Docker)

## Environment Variables

Create a `.env` file in the root directory with the following structure:

```env
HOST=0.0.0.0
PORT=8000
ENV=development
GENDERIZED_API_BASE_URL=https://api.genderize.io
AGIFY_API_BASE_URL=https://api.agify.io
NATIONAIZE_API_BASE_URL=https://api.nationalize.io
DB_URL=postgres://admin:secret@db:5432/gender_api?sslmode=disable
REDIS_URL=redis:6379
```
*(Note: Change `db` and `redis` hosts to `localhost` if running the Go app natively outside of Docker).*

## Running the Application

### The Easy Way (Docker Compose)

The simplest way to run the entire stack (API, PostgreSQL, and Redis) is using Docker Compose:

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd gender-api
   ```
2. Start the services:
   ```bash
   docker-compose up --build
   ```

The API will be available at `http://localhost:8000`. Database migrations will run automatically on startup.

### Running Natively (Go + External DB/Redis)

If you prefer to run the Go application natively, ensure you have PostgreSQL and Redis instances running.

1. Install dependencies:
   ```bash
   go mod download
   ```
2. Apply migrations using Goose:
   ```bash
   goose -dir internals/sql/migrations postgres "postgres://<user>:<pass>@localhost:5432/gender_api?sslmode=disable" up
   ```
3. Start the server:
   ```bash
   go run ./cmd
   ```

## API Documentation

### 1. Create Profile

Generates a new profile for a given name, caches it, and stores it in the database.

**Endpoint:** `POST /api/profiles`

**Request Body:**
```json
{
  "name": "peter"
}
```

**Success Response (201 Created or 200 OK if already exists):**
```json
{
  "status": "success",
  "data": {
    "id": "019da188-7d51-72c8-aac6-8462cbc09a3e",
    "name": "peter",
    "gender": "male",
    "gender_probability": 0.99,
    "sample_size": 165452,
    "age": 42,
    "age_group": "adult",
    "country_id": "US",
    "country_probability": 0.08,
    "created_at": "2024-04-18T16:59:30.003844Z"
  }
}
```

### 2. Get Profile by ID

Retrieves a specific profile by its UUID.

**Endpoint:** `GET /api/profiles/{id}`

**Success Response (200 OK):**
```json
{
  "status": "success",
  "data": {
    "id": "019da188-7d51-72c8-aac6-8462cbc09a3e",
    "name": "peter",
    "gender": "male",
    ...
  }
}
```

### 3. List Profiles

Retrieves a paginated/filtered list of stored profiles (returns a simplified profile view).

**Endpoint:** `GET /api/profiles`

**Query Parameters (Optional):**
- `gender` (string): Filter by gender (e.g., `male`, `female`).
- `country_id` (string): Filter by 2-letter country code (e.g., `US`, `GH`).
- `age_group` (string): Filter by age group (`child`, `teenager`, `adult`, `senior`).

**Success Response (200 OK):**
```json
{
  "status": "success",
  "count": 1,
  "data": [
    {
      "id": "019da188-7d51-72c8-aac6-8462cbc09a3e",
      "name": "peter",
      "gender": "male",
      "age": 42,
      "age_group": "adult",
      "country_id": "US"
    }
  ]
}
```

### 4. Delete Profile

Removes a profile from both the database and the Redis cache.

**Endpoint:** `DELETE /api/profiles/{id}`

**Success Response (204 No Content)**
*(No body returned)*
