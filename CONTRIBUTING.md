# Contributing to requests

Contributions to `requests` are welcome when they keep the API small, explicit,
and easy to reason about.

## How to Contribute

### Reporting Issues

Before submitting an issue, please check the issue tracker to avoid duplicates. When creating an issue, provide as much information as possible to help us understand and address the problem quickly.

### Submitting Patches

1. **Fork the repository** on GitHub.
2. **Clone your fork** to your local machine.
3. **Create a new branch** for your change.
4. **Make your change**. Keep code clear, tested, and narrowly scoped.
5. **Commit your change**. Use a clear commit message.
6. **Push your changes** to your fork on GitHub.
7. **Submit a pull request**. Include a clear description of the changes and any relevant issue numbers.

Run `task test` plus `task lint` for root-only changes. Run `task test:all`
plus `task lint:all` when a change touches extension modules or shared
contracts.

### Code Style

Follow the conventions used throughout the project. Prefer clear names,
accurate comments, and small public surfaces.

### Adding Documentation

Documentation changes are as valuable as code changes when they clarify real
behavior for users or maintainers.

## Conduct

Keep discussion respectful, specific, and focused on the code or documentation
under review.

## Questions?

If you have any questions about contributing, please reach out by opening an issue or contacting the project maintainers directly.

Thank you for your interest in contributing to `requests`.
