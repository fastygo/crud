# Go Fast CMS - A Lightweight CRUD Example

A lightweight Content Management System (CMS) built with Go, focusing on high performance using `fasthttp` and `quicktemplate`. This project serves as a practical example demonstrating full CRUD operations, JSON import/export, and a clean project structure, making it an excellent starting point for junior Go developers.

## Features

*   **High Performance:** Built entirely in Go and leverages the blazing-fast `fasthttp` library for handling HTTP requests with minimal overhead and allocations. Aims for response times in the **2-4 millisecond** range for core API and page generation logic (excluding network latency).
*   **Efficient Templating:** Uses `quicktemplate` (qtc) for generating HTML. Templates are precompiled into Go code, eliminating runtime template parsing bottlenecks and further boosting performance.
*   **Ephemeral Embedded Database:** Utilizes `bbolt` for data storage. The database is initialized from an embedded file on startup and runs ephemerally (data persists only for the application's lifetime), making it easy to run and experiment without external database dependencies.
*   **Full CRUD API:** Provides a complete JSON API for managing content items:
    *   `GET /api/content`: List all items.
    *   `GET /api/content/{id}`: Get a specific item.
    *   `POST /api/content`: Create a new item.
    *   `PUT /api/content/{id}`: Update an existing item.
    *   `DELETE /api/content/{id}`: Delete an item.
*   **Server-Rendered HTML:** Generates HTML pages on the server using the precompiled `quicktemplate` templates for common CMS views (List, View, Create, Edit).
*   **JSON Import/Export:** Includes API endpoints for easily exporting the entire content database to JSON (`POST /api/export`) and importing content from a JSON file (`POST /api/import`), replacing existing data.
*   **Minimalist Frontend:** Relies on CDN-delivered assets for styling and basic interactivity:
    *   **Tailwind CSS v4 (via Browser CDN):** Provides modern utility-first styling.
    *   **Alpine.js (via CDN):** Used for simple frontend interactions (like mobile menu toggles).
    *   **Zero Server Impact:** Loading these CDN assets happens entirely in the user's browser, contributing **0 ms** to the Go application's server response time.
*   **Clear Project Structure:** Follows standard Go practices (`cmd/`, `internal/`) with logical separation of concerns (handlers, storage, core, templates, models).

## Technology Stack

*   **Language:** Go (1.21+)
*   **Web Server:** `fasthttp`
*   **Routing:** `fasthttp/router` (wrapped in `internal/core`)
*   **Templating:** `quicktemplate` (qtc)
*   **Database:** `bbolt` (embedded)
*   **Frontend:** Tailwind CSS v4 (CDN), Alpine.js (CDN)

## Performance

This project prioritizes speed. By using `fasthttp`, which is designed for high-throughput scenarios with low memory allocations, and `quicktemplate`, which compiles templates to efficient Go code, the core application logic aims for response times typically between **2-4 milliseconds**.

The use of CDNs for Tailwind CSS and Alpine.js ensures that the Go backend is not involved in serving these assets, keeping its focus solely on fast data processing and HTML generation.

## Architecture

The project adheres to a standard Go layout:

*   `cmd/cms/main.go`: Application entry point, server initialization, and routing setup.
*   `internal/`: Contains the core application logic:
    *   `config/`: Application configuration loading.
    *   `core/`: Request router wrapper.
    *   `handlers/`: HTTP request handlers (API, Pages, Static - *currently inactive*).
    *   `storage/`: Database interaction logic (`bbolt`).
    *   `templates/`: `quicktemplate` source files (`.qtpl`) and generated Go code.
    *   `models/`: Data structures and template view models.
*   `cmd/cms/assets/`: Embedded assets (database initial state, *inactive* static files).

## Target Audience

This project is an ideal learning resource and starting point for **junior Go developers** who want to:

*   Understand how to build a web application in Go.
*   See a practical implementation of CRUD operations.
*   Learn about high-performance HTTP handling with `fasthttp`.
*   Explore efficient server-side templating with `quicktemplate`.
*   Grasp a standard Go project structure.
*   Build a functional application with API and HTML interfaces.

## Getting Started

### Prerequisites

*   Go (version 1.21 or later recommended)
*   `quicktemplate` compiler (`qtc`)

   ```bash
   go install github.com/valyala/quicktemplate/qtc@latest
   ```

### Running the Application

1.  **Clone the repository:**
    ```bash
    git clone <repository_url>
    cd <repository_directory>
    ```
2.  **Generate Go code from templates:**
    ```bash
    qtc -dir=internal/templates
    ```
3.  **Build the application:**
    ```bash
    go build -o cms ./cmd/cms
    ```
    *(Alternatively, run directly)*
    ```bash
    go run ./cmd/cms/main.go
    ```
4.  **Run the executable:**
    ```bash
    ./cms
    ```
5.  **Access the application:** Open your web browser to `http://localhost:8080`

## Future Improvements

*   Implement a robust static file serving solution (revisiting the `internal/handlers/static.go` logic).
*   Add user authentication and authorization.
*   Integrate a build pipeline for CSS/JS instead of relying solely on CDNs.
*   Introduce database migrations.
*   Add unit and integration tests.
