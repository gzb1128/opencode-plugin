# Contributing to OpenCode Plugin CLI

Thank you for your interest in contributing!

## Development Setup

```bash
# Clone the repository
git clone <repository-url>
cd opencode-plugin

# Build
make build

# Run tests
make test

# Run e2e tests
make test-e2e
```

## Pull Request Process

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes and add tests if applicable
4. Ensure all tests pass (`make test`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## Code Style

- Follow existing code conventions in the project
- Write meaningful commit messages
- Add comments for complex logic
- Ensure code is properly formatted (`go fmt`)

## Reporting Issues

Use GitHub Issues to report bugs or suggest features. Include:
- Clear description
- Steps to reproduce (for bugs)
- Environment details
