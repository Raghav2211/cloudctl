# cloudctl
cloudctl is a GO library that interacts with cloud providers and displays output in a human-readable fashion.

## Running locally

### Prerequisites
- Go 1.22+
- AWS credentials configured (e.g. via `~/.aws/credentials`, `~/.aws/config`, or `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` / `AWS_REGION` env vars) — cloudctl uses the default AWS SDK credential chain

### Build and run

```bash
# Fetch dependencies
go mod download

# Build the binary
go build -o cloudctl .

# Run it
./cloudctl --help
```

Or run directly without building:

```bash
go run . --help
```

### Example commands

```bash
# List S3 buckets
./cloudctl aws s3 ls

# List objects in a bucket
./cloudctl aws s3 list-objects --name <bucket-name>

# List EC2 instances
./cloudctl aws ec2 ls

# Get EC2 instance statistics
./cloudctl aws ec2 stats
```
