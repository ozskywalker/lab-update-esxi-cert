# Contributing to ESXi Certificate Manager

Thank you for your interest in contributing to this project! This guide will help you understand our development process and contribution requirements.

## Conventional Commits

This project uses [Conventional Commits](https://www.conventionalcommits.org/) for automated changelog generation and semantic versioning. All commit messages must follow this format:

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### Commit Types

The following commit types are recognized by our automated changelog system:

- **`feat:`** - New features (appears in "Features" section)
- **`fix:`** - Bug fixes (appears in "Bug fixes" section)  
- **`sec:`** - Security-related changes (appears in "Security" section)
- **`perf:`** - Performance improvements (appears in "Performance" section)
- **`docs:`** - Documentation changes (filtered out of changelog)
- **`test:`** - Test changes (filtered out of changelog)
- **`build:`** - Build system changes (filtered out of changelog)
- **`ci:`** - CI/CD changes (filtered out of changelog)
- **`refactor:`** - Code refactoring (filtered out of changelog)
- **`style:`** - Code style changes (filtered out of changelog)

### Examples

**Good commit messages:**
```
feat: add support for certificate key size configuration
fix: resolve SOAP authentication failure with special characters
sec: validate AWS credentials before certificate operations
perf: optimize certificate validation checks
docs: update README with new configuration options
```

**Bad commit messages:**
```
Update code
Fix bug
Add feature
Changed stuff
```

### Breaking Changes

For breaking changes, add `BREAKING CHANGE:` in the footer or use `!` after the type:

```
feat!: change default renewal threshold to 0.25

BREAKING CHANGE: Default renewal threshold changed from 0.33 to 0.25
```

## Development Workflow

1. **Fork the repository** and create a feature branch
2. **Make your changes** following the code style
3. **Test your changes** using `go test ./...`
4. **Format your code** using `go fmt ./...`
5. **Vet your code** using `go vet ./...`
6. **Commit your changes** using conventional commit format
7. **Push to your fork** and create a pull request

## Code Guidelines

- Follow standard Go conventions and formatting
- Add tests for new functionality
- Update documentation as needed
- Ensure all existing tests pass
- Keep commits focused and atomic

## Pull Request Process

1. Ensure your branch is up to date with the main branch
2. Include a clear description of the changes
3. Reference any related issues
4. Ensure all CI checks pass
5. Request review from maintainers

## Questions?

Feel free to open an issue for any questions about contributing or to discuss potential changes before implementing them.