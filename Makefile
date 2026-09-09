.PHONY: json test recommend serve eval

json:
	python3 scripts/csv_to_json.py

test:
	go test ./...

eval:
	go test -v ./internal/eval

recommend:
	go run ./cmd/shelfmate recommend -student S-406

serve:
	go run ./cmd/shelfmate serve -addr :8088
