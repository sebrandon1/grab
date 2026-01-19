# Contributing to Grab

Thank you for your interest in contributing to Grab! This document provides guidelines and instructions for contributing.

## Getting Started

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/grab.git
   cd grab
   ```
3. Add the upstream remote:
   ```bash
   git remote add upstream https://github.com/sebrandon1/grab.git
   ```

### Branch Naming Conventions

Use descriptive branch names with the following prefixes:

- `feature/` - New features (e.g., `feature/resume-downloads`)
- `fix/` - Bug fixes (e.g., `fix/progress-bar-overflow`)
- `docs/` - Documentation updates (e.g., `docs/update-readme`)
- `refactor/` - Code refactoring (e.g., `refactor/client-options`)

## Code Style

### Formatting

- Run `go fmt` before committing:
  ```bash
  go fmt ./...
  ```

### Linting

- Run `golangci-lint` to check for issues:
  ```bash
  make lint
  ```
- Address all linting errors before submitting a PR

### General Guidelines

- Follow standard Go conventions and idioms
- Keep functions focused and concise
- Add comments for exported functions and complex logic
- Use meaningful variable and function names

## Testing

### Running Tests

All changes must pass existing tests:

```bash
make test
```

Or directly with Go:

```bash
go test ./...
```

### Writing Tests

- Add tests for new functionality
- Update tests when modifying existing behavior
- Aim for clear, readable test cases

## Pull Request Process

### Before Submitting

1. Sync with upstream:
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```
2. Run tests: `make test`
3. Run linting: `make lint`
4. Ensure your code builds: `make build`

### Submitting a PR

1. Push your branch to your fork
2. Open a Pull Request against `main`
3. Fill out the PR template completely
4. Link any related issues

### PR Review

- Address reviewer feedback promptly
- Keep the PR focused on a single change
- Squash fixup commits before merging

## Commit Message Guidelines

Write clear, concise commit messages:

- Use the imperative mood ("Add feature" not "Added feature")
- Keep the first line under 72 characters
- Reference issues when applicable (e.g., "Fix progress display (#123)")

### Examples

```
Add retry logic for failed downloads

Implement exponential backoff when downloads fail due to
network errors. Configurable via --retries flag.

Fixes #42
```

```
Fix incorrect file size calculation

The Content-Length header was being parsed incorrectly for
files larger than 2GB on 32-bit systems.
```

## Questions?

If you have questions about contributing, feel free to open an issue for discussion.
