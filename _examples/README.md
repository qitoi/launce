# examples

This directory contains example scripts for launce.
Each example contains:

- `*.go`: Worker script
- `locustfile.py`: Master script
- `locustfile.worker.py`: Worker script for Locust that is equivalent to Go worker script

## Usage

Run Master script.

```sh
locust --master -f locustfile.py
```

Run worker script.

```sh
go run .
```

Or run locust worker script.

```sh
locust --worker -f locustfile.worker.py
```
