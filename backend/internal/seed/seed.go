package seed

import (
	"time"

	"snippetsync/backend/internal/models"
)

func Modules() []models.Module {
	now := time.Now().UTC()
	return []models.Module{
		module("jwt-auth", "JWT Authentication", "Python", "Flask", "Reusable login, token issuing, auth middleware, and protected-route tests.", []string{"Python", "Flask", "Auth", "Production Ready"}, []string{"logging-framework"}, now, []models.ModuleFile{
			file("auth/routes.py", "from flask import Blueprint, jsonify\n\nbp = Blueprint('auth', __name__)\n\n@bp.post('/login')\ndef login():\n    return jsonify({'access_token': 'demo-token'})\n", "python"),
			file("auth/middleware.py", "def require_auth(handler):\n    def wrapped(*args, **kwargs):\n        return handler(*args, **kwargs)\n    return wrapped\n", "python"),
			file("auth/README.md", "# JWT Authentication\n\nDrop-in Flask auth module with route and middleware boundaries.\n", "markdown"),
		}),
		module("postgresql-setup", "PostgreSQL Setup", "Python", "Flask", "SQLAlchemy engine, migrations, health check, and local connection defaults.", []string{"Database", "PostgreSQL", "SQLAlchemy"}, []string{"docker-config"}, now, []models.ModuleFile{
			file("database/db.py", "from sqlalchemy import create_engine\n\nengine = create_engine('postgresql://app:app@db:5432/app')\n", "python"),
			file("database/health.py", "def check_database():\n    return {'database': 'ok'}\n", "python"),
		}),
		module("docker-config", "Docker Configuration", "Docker", "Compose", "Dockerfile and compose baseline for local backend development.", []string{"Docker", "DevOps", "Backend"}, nil, now, []models.ModuleFile{
			file("Dockerfile", "FROM python:3.12-slim\nWORKDIR /app\nCOPY . .\nCMD [\"python\", \"app.py\"]\n", "dockerfile"),
			file("docker-compose.yml", "services:\n  api:\n    build: .\n    ports:\n      - \"5000:5000\"\n", "yaml"),
		}),
		module("kafka-producer", "Kafka Producer", "Python", "FastAPI", "Typed event producer with retry hooks and topic configuration.", []string{"Kafka", "Events", "Producer"}, []string{"logging-framework"}, now, []models.ModuleFile{
			file("kafka/producer.py", "def publish(topic, payload):\n    print(f'publishing {topic}: {payload}')\n", "python"),
		}),
		module("kafka-consumer", "Kafka Consumer", "Python", "FastAPI", "Consumer loop with retry middleware and dead-letter queue boundary.", []string{"Kafka", "Events", "Consumer"}, []string{"kafka-producer", "logging-framework"}, now, []models.ModuleFile{
			file("kafka/consumer.py", "def consume(topic):\n    print(f'consuming {topic}')\n", "python"),
		}),
		module("logging-framework", "Logging Framework", "Python", "Any", "Structured logging helpers with request context and JSON formatting.", []string{"Logging", "Observability", "Production Ready"}, nil, now, []models.ModuleFile{
			file("logging_config.py", "import logging\n\nlogging.basicConfig(level=logging.INFO, format='%(message)s')\n", "python"),
		}),
		module("redis-cache", "Redis Cache", "Python", "Flask", "Cache client, TTL helpers, and cache-aside wrapper.", []string{"Redis", "Cache", "Performance"}, []string{"docker-config"}, now, []models.ModuleFile{
			file("cache/redis_client.py", "class Cache:\n    def get(self, key):\n        return None\n", "python"),
		}),
		module("rate-limiter", "Rate Limiter", "Python", "Flask", "IP-based request limiter designed to pair with Redis.", []string{"Security", "Redis", "Middleware"}, []string{"redis-cache"}, now, []models.ModuleFile{
			file("middleware/rate_limit.py", "def rate_limit(handler):\n    def wrapped(*args, **kwargs):\n        return handler(*args, **kwargs)\n    return wrapped\n", "python"),
		}),
	}
}

func module(id, title, language, framework, description string, tags, deps []string, now time.Time, files []models.ModuleFile) models.Module {
	return models.Module{
		ID:           id,
		Title:        title,
		Language:     language,
		Framework:    framework,
		Description:  description,
		Tags:         tags,
		Dependencies: deps,
		Files:        files,
		Versions: []models.ModuleVersion{{
			Version:   "v1.0.0",
			Message:   "Initial battle-tested module import",
			Files:     files,
			CreatedAt: now,
		}},
		Owner:     "demo-user",
		Favorite:  id == "jwt-auth" || id == "docker-config",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func file(path, content, language string) models.ModuleFile {
	return models.ModuleFile{Path: path, Content: content, Language: language}
}
