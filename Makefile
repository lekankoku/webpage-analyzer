.PHONY: dev dev-server dev-frontend

dev:
	@set -e; \
	( cd server && go run ./cmd/app ) & \
	server_pid=$$!; \
	( cd frontend && pnpm dev ) & \
	frontend_pid=$$!; \
	trap 'kill $$server_pid $$frontend_pid 2>/dev/null || true' INT TERM EXIT; \
	wait

dev-server:
	@cd server && go run ./cmd/app

dev-frontend:
	@cd frontend && pnpm dev
