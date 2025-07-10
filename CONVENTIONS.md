# Go Coding Conventions

## Package Management

- Use go modules for dependency management
- Keep go.mod and go.sum in version control
- Prefer direct dependencies over transitive ones

## Code Style

- Follow golint rules
- Use consistent indentation (tabs preferred)
- Maximum line length: 80 characters
- Error handling required for all operations that can fail

## Package Organization

- One package per directory
- Clear package naming conventions
- Public interfaces defined in separate files

## Testing

- Write unit tests for all public functions
- Use table-driven tests where appropriate
- Maintain test coverage above 80%

## Documentation

- Add comments for exported functions
- Document complex algorithms
- Explain non-obvious design decisions
