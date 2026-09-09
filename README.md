# Forum

A clean, responsive web forum application built in **Go** and backed by **SQLite**. The forum enables registered users to create posts, categorize discussions, leave comments, and interact through likes and dislikes, while allowing guests to browse content freely.

---

## Features

- **User Authentication**:
  - Secure registration and login with email, username, and password.
  - Password encryption using `bcrypt`.
  - Secure, HTTP-only cookie-based session management.
- **Posts & Categorization**:
  - Create posts with titles, content, and multiple category tags.
  - Filter posts by category.
  - Filter posts created by the logged-in user.
  - Filter posts liked by the logged-in user.
- **Comments**:
  - Comment on posts with full author and timestamp attribution.
- **Likes & Dislikes**:
  - React to posts and comments with likes or dislikes.
  - Enforced single reaction per user per post/comment.
- **Robust SQLite Database Layer**:
  - Relational schema with foreign keys (`PRAGMA foreign_keys = ON`).
  - Automatic cascading deletions on user/post removals.
  - Write-Ahead Logging (WAL) enabled for concurrency and performance.
  - Embedded schema migrations using Go's `embed` package.

---

## Tech Stack

- **Language**: Go (v1.26+)
- **Database**: SQLite3 via [`github.com/mattn/go-sqlite3`](https://github.com/mattn/go-sqlite3)
- **Hashing**: `golang.org/x/crypto/bcrypt`
- **Identifier Generation**: UUIDv4 via [`github.com/gofrs/uuid`](https://github.com/gofrs/uuid)
- **Frontend**: HTML5, CSS3, JavaScript (Vanilla ES6)

---

## Project Structure

```text
.
├── cmd/
│   └── server/
│       └── main.go           # Application entrypoint & HTTP server routing
├── internal/
│   ├── auth/                 # User registration, login, logout & session middleware
│   ├── categories/           # Category querying and listing
│   ├── comments/             # Comment creation and retrieval
│   ├── db/                   # SQLite connection, migration, schema & tests
│   │   ├── db.go             # Database connection & lifecycle management
│   │   ├── db_test.go        # Database unit tests
│   │   └── schema.sql        # Database DDL schema, indexes & seed data
│   ├── httputil/             # JSON encoding/decoding and error helpers
│   ├── posts/                # Post creation, listing, and filtering
│   └── reactions/            # Reaction logic for posts and comments
├── web/
│   ├── static/               # Frontend static assets (CSS, JS)
│   │   ├── css/style.css     # Responsive forum styles
│   │   └── js/               # Frontend API client and application scripts
│   └── templates/            # HTML templates
├── docker-compose.yml        # Container orchestration
├── go.mod                    # Go module definition
├── go.sum                    # Dependency checksums
└── README.md                 # Project documentation
```

---

## Database Architecture

The SQLite database is managed in `internal/db` and automatically initializes tables, indexes, and seed categories upon startup:

| Table | Description |
|---|---|
| `users` | Stores user credentials (`id`, `email`, `username`, `password_hash`, `created_at`). |
| `sessions` | Active user sessions with expiration (`id`, `user_id`, `expires_at`). |
| `posts` | Discussion posts created by users (`id`, `user_id`, `title`, `content`, `created_at`). |
| `categories` | Forum topic tags (`id`, `name`). Default seeds: *General*, *Technology*, *Discussions*, *Feedback*. |
| `post_categories` | Many-to-many junction mapping posts to categories. |
| `comments` | User comments under posts (`id`, `post_id`, `user_id`, `content`, `created_at`). |
| `reactions` | User reactions on posts or comments (`id`, `user_id`, `post_id`, `comment_id`, `type`, `created_at`). |

---

## API Endpoints

### Authentication
- `POST /api/register` - Register a new account (`email`, `username`, `password`).
- `POST /api/login` - Authenticate and establish a session cookie.
- `POST /api/logout` - Invalidate session and clear session cookie.
- `GET /api/me` - Get current authenticated user profile.

### Categories
- `GET /api/categories` - List all available categories.

### Posts
- `GET /api/posts` - List posts (supports `?category=<name>` and `?filter=created|liked`).
- `GET /api/posts/{id}` - Fetch a single post with its category tags and reaction counts.
- `POST /api/posts` - Create a new post *(Requires authentication)*.

### Comments
- `GET /api/posts/{id}/comments` - List all comments on a post.
- `POST /api/posts/{id}/comments` - Create a comment on a post *(Requires authentication)*.

### Reactions
- `POST /api/posts/{id}/react` - Like or dislike a post *(Requires authentication)*.
- `POST /api/comments/{id}/react` - Like or dislike a comment *(Requires authentication)*.

---

## Getting Started

### Prerequisites
- **Go**: Version 1.22 or newer.
- **GCC**: Required by `go-sqlite3` (CGO).
  ```bash
  # Debian / Ubuntu
  sudo apt-get install gcc
  ```

### Installation

1. **Clone the repository:**
   ```bash
   git clone https://learn.zone01kisumu.ke/git/tabdalla/Forum.git
   cd Forum
   ```

2. **Download dependencies:**
   ```bash
   go mod download
   ```

3. **Run the server:**
   ```bash
   go run ./cmd/server
   ```

4. Open your browser at [http://localhost:8080](http://localhost:8080).

### Environment Variables

| Variable | Description | Default |
|---|---|---|
| `PORT` | HTTP port the server listens on | `8080` |
| `DB_PATH` | File path for SQLite database | `./data/forum.db` |

---

## Running Tests

Run all unit tests across the codebase:

```bash
# Run database tests
go test -v ./internal/db/...

# Run auth tests
go test -v ./internal/auth/...

# Run post tests
go test -v ./internal/posts/...

# Run all tests
go test -v ./...
```
