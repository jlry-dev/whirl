# Whirl Backend

A real-time chat application backend built with Go, featuring WebSocket-based messaging, user authentication, friendship management, and random chat pairing functionality.

## 🚀 Features

### Core Functionality
- **User Authentication**: Secure registration and login with JWT-based authentication
- **Real-time Messaging**: WebSocket-powered instant messaging between users
- **Friendship System**: Manage friend connections with status tracking (accepted/blocked)
- **Random Chat Pairing**: Match users randomly for spontaneous conversations
- **Avatar Management**: Upload and manage user profile avatars with Cloudinary integration
- **Country Support**: Multi-country user registration with ISO 3166-1 alpha-3 codes

### Technical Highlights
- Clean architecture with separation of concerns (handlers, services, repositories)
- PostgreSQL database with connection pooling
- Image hash-based duplicate avatar detection
- Comprehensive input validation
- Structured logging with `slog`
- CORS middleware for cross-origin requests
- Database migrations support

## 📋 Prerequisites

- **Go**: 1.24.6 or higher
- **Docker**: For PostgreSQL container
- **golang-migrate**: For database migrations
- **PostgreSQL**: 16.10 (via Docker)

## 🛠️ Installation

### 1. Clone the Repository
```bash
git clone https://github.com/jlry-dev/whirl.git
cd whirl-backend
```

### 2. Set Up Environment Variables
Create a `.env` file in the project root with the following variables:

```env
# Database Configuration
POSTGRES_USER=your_db_user
POSTGRES_PASSWORD=your_db_password
POSTGRES_DB=whirl_db
DATABASE_CONN_STR=postgresql://your_db_user:your_db_password@localhost:5432/whirl_db?sslmode=disable

# Server Configuration
SERVER_ADDRESS=:8080

# JWT Configuration
JWT_SECRET=your_jwt_secret_key

# Cloudinary Configuration (for avatar uploads)
CLOUDINARY_CLOUD_NAME=your_cloud_name
CLOUDINARY_API_KEY=your_api_key
CLOUDINARY_API_SECRET=your_api_secret
```

### 3. Initialize Database
```bash
# Create and start PostgreSQL container
make init-db

# Run database migrations
make migrate-db-up
```

### 4. Install Dependencies
```bash
go mod download
```

### 5. Run the Server
```bash
go run cmd/server/main.go
```

The server will start at the address specified in `SERVER_ADDRESS` (default: `:8080`).

## 📁 Project Structure

```
whirl-backend/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
├── internal/
│   ├── config/                  # Configuration management
│   │   ├── database.go          # Database connection setup
│   │   └── server.go            # Server configuration
│   ├── handler/                 # HTTP/WebSocket handlers
│   │   ├── auth.go              # Authentication endpoints
│   │   ├── chat.go              # WebSocket chat hub & client
│   │   ├── friendship.go        # Friendship management
│   │   ├── message.go           # Message retrieval
│   │   ├── user.go              # User profile management
│   │   └── response.go          # Response utilities
│   ├── middleware/              # HTTP middleware
│   │   └── middleware.go        # Auth & CORS middleware
│   ├── model/                   # Data models & DTOs
│   │   ├── user.go              # User model
│   │   ├── friendship.go        # Friendship model
│   │   ├── message.go           # Message model
│   │   ├── avatar.go            # Avatar model
│   │   └── country.go           # Country model
│   ├── repository/              # Data access layer
│   │   ├── repository.go        # Repository interfaces
│   │   ├── postgres_user.go     # User repository
│   │   ├── postgres_friendship.go
│   │   ├── postgres_message.go
│   │   ├── postgres_avatar.go
│   │   └── postgres_country.go
│   ├── service/                 # Business logic layer
│   │   ├── auth.go              # Authentication service
│   │   ├── user.go              # User service
│   │   ├── friendship.go        # Friendship service
│   │   └── message.go           # Message service
│   └── util/                    # Utility functions
│       ├── jwt.go               # JWT token generation/validation
│       └── custom_validators.go # Custom validation rules
├── db/
│   └── migrations/              # Database migration files
│       ├── 000001_postgres_migration.up.sql
│       └── 000001_postgres_migration.down.sql
├── test/                        # Unit tests
│   ├── mocks/                   # Mock implementations
│   └── unit/
│       └── service/             # Service layer tests
├── Makefile                     # Build and database commands
├── go.mod                       # Go module dependencies
└── README.md                    # This file
```

## 🔌 API Endpoints

### Authentication
- `POST /auth/register` - Register a new user
  - Body: `{ username, email, password, bio, birthDate, countryCode }`
  - Returns: JWT token and user details

- `POST /auth/login` - Login existing user
  - Body: `{ username, password }`
  - Returns: JWT token and user details

### User Management
- `POST /user/avatar` - Upload/update user avatar (authenticated)
  - Requires: JWT token in Authorization header
  - Body: Multipart form data with image file

### Friendship
- `GET /friends` - Retrieve user's friends list (authenticated)
- `PUT /friend` - Update friendship status (authenticated)
  - Body: `{ friendId, status }`
- `DELETE /friend` - Remove a friend (authenticated)
  - Body: `{ friendId }`

### Messaging
- `GET /messages/{id}` - Retrieve message history with a specific user (authenticated)
  - Path parameter: `id` - User ID to retrieve messages with

### WebSocket
- `GET /websocket/connect` - Establish WebSocket connection (authenticated)
  - Requires: JWT token in Authorization header
  - Supports real-time messaging and random chat pairing

## 🔐 Authentication Flow

1. **Registration**:
   - User submits registration data
   - Password is hashed using bcrypt
   - User record created in database
   - JWT token generated and returned

2. **Login**:
   - User submits credentials
   - Password verified against stored hash
   - JWT token generated and returned

3. **Authenticated Requests**:
   - Client includes JWT token in `Authorization` header
   - Middleware validates token and extracts user ID
   - User ID injected into request context for handlers

## 💬 WebSocket Chat System

### Hub Architecture
The chat system uses a centralized Hub pattern:
- **Hub**: Manages all connected clients and message routing
- **Client**: Represents individual WebSocket connections
- **Message Types**:
  - `connect` - Client connection established
  - `disconnect` - Client disconnection
  - `message` - Text message between users
  - `random_join` - Join random chat queue
  - `random_leave` - Leave random chat queue
  - `friend_request` - Send friend request
  - `friend_accept` - Accept friend request
  - `friend_block` - Block a user

### Random Chat Pairing
- Users can join a random chat queue
- System automatically pairs users when available
- Paired users can exchange messages in real-time
- Either user can leave the random chat at any time

## 🗄️ Database Schema

### Tables
- **app_user**: User accounts and profiles
- **country**: Supported countries (ISO 3166-1 alpha-3)
- **avatar**: User avatar metadata and Cloudinary references
- **friendship**: User relationships with status tracking
- **message**: Chat message history

### Key Relationships
- Users belong to a country
- Users can have one avatar
- Friendships are bidirectional (user1 ↔ user2)
- Messages link sender and receiver users

## 🧪 Testing

Run unit tests:
```bash
go test ./test/unit/...
```

Run tests with coverage:
```bash
go test -cover ./test/unit/...
```

Generate coverage report:
```bash
go test -coverprofile=coverage.out ./test/unit/...
go tool cover -html=coverage.out
```

## 🔧 Makefile Commands

```bash
make init-db          # Create and start PostgreSQL Docker container
make start-db         # Start existing PostgreSQL container
make stop-db          # Stop PostgreSQL container
make migrate-db-up    # Run database migrations (up)
make migrate-db-down  # Rollback database migrations (down)
```

## 📦 Key Dependencies

- **gorilla/websocket**: WebSocket implementation
- **jackc/pgx/v5**: PostgreSQL driver and connection pooling
- **golang-jwt/jwt/v5**: JWT token generation and validation
- **go-playground/validator/v10**: Request validation
- **cloudinary-go**: Avatar upload and management
- **ajdnik/imghash**: Perceptual image hashing for duplicate detection
- **golang.org/x/crypto**: Password hashing (bcrypt)

## 🏗️ Architecture

### Layered Architecture
```
┌─────────────────────────────────────┐
│         HTTP/WebSocket Layer        │
│  (Handlers, Middleware, Routing)    │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│        Business Logic Layer         │
│         (Services)                  │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│        Data Access Layer            │
│        (Repositories)               │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│          PostgreSQL Database        │
└─────────────────────────────────────┘
```

### Design Patterns
- **Repository Pattern**: Abstracts data access logic
- **Service Pattern**: Encapsulates business logic
- **Dependency Injection**: Loose coupling between layers
- **Interface-based Design**: Enables testing with mocks
- **Hub Pattern**: Centralized WebSocket connection management

## 🔒 Security Features

- **Password Hashing**: Bcrypt with default cost factor
- **JWT Authentication**: Secure token-based authentication
- **Input Validation**: Comprehensive request validation
- **SQL Injection Prevention**: Parameterized queries via pgx
- **CORS Protection**: Configurable CORS middleware
- **Image Hash Verification**: Prevents duplicate avatar uploads

## 🚦 Error Handling

The application uses structured error handling:
- **Validation Errors**: Detailed field-level validation messages
- **Service Errors**: Domain-specific error types
- **Repository Errors**: Database operation errors
- **HTTP Status Codes**: Appropriate status codes for all responses

## 📝 Logging

Structured logging using Go's `slog` package:
- Request/response logging
- Error logging with context
- WebSocket connection events
- Database operation logs

## 🙏 Acknowledgments

- Built with Go's standard library and carefully selected third-party packages
- Inspired by modern real-time chat applications
- Database schema designed for scalability and performance
