.PHONY: proto test test-proto build tidy ps logs migrate-auth migrate-booking migrate-business migrate-billing migrate-notification migrate-analytics migrate-scheduler bootstrap-topics clean-protos

proto:
	./scripts/gen-proto.sh

test:
	go test ./...

test-proto:
	go test -tags protogen ./...

build:
	go build ./...

tidy:
	go mod tidy

ps:
	docker compose -f deploy/compose/docker-compose.yml ps

logs:
	docker compose -f deploy/compose/docker-compose.yml logs -f --tail=200

migrate-auth:
	DATABASE_URL=postgres://auth_user:auth_password@localhost:5432/auth_db?sslmode=disable ./scripts/migrate-auth.sh

migrate-booking:
	DATABASE_URL=postgres://booking_user:booking_password@localhost:5432/booking_db?sslmode=disable ./scripts/migrate-booking.sh

migrate-business:
	DATABASE_URL=postgres://business_user:business_password@localhost:5432/business_db?sslmode=disable ./scripts/migrate-business.sh

migrate-billing:
	DATABASE_URL=postgres://billing_user:billing_password@localhost:5432/billing_db?sslmode=disable ./scripts/migrate-billing.sh

migrate-notification:
	DATABASE_URL=postgres://notification_user:notification_password@localhost:5432/notification_db?sslmode=disable ./scripts/migrate-notification.sh

migrate-analytics:
	DATABASE_URL=postgres://analytics_user:analytics_password@localhost:5432/analytics_db?sslmode=disable ./scripts/migrate-analytics.sh

migrate-scheduler:
	DATABASE_URL=postgres://scheduler_user:scheduler_password@localhost:5432/scheduler_db?sslmode=disable ./scripts/migrate-scheduler.sh

bootstrap-topics:
	./scripts/bootstrap-topics.sh

clean-protos:
	./scripts/clean-protos.sh
