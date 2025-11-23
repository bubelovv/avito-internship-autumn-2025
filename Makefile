.PHONY: test fmt tidy up down

up:
	docker-compose up --build

down:
	docker-compose down -v
