# Test Data Fixtures

This directory contains test data and fixtures used by the test suite.

## Structure

- `configs/` - Sample configuration files for testing configuration parsing
- `certificates/` - Test certificates (generated during test runs)
- `fixtures/` - Other test fixtures and data

## Usage

Test certificates and other dynamic fixtures are typically generated during test execution using the testutil package helper functions rather than being stored as static files.

Configuration files are used to test the configuration parsing and validation logic under various scenarios including valid configurations, invalid configurations, and malformed JSON.