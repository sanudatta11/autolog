# Test Cases Management

The enhanced ML logparser now uses a JSON-based test case system that makes it easy to maintain and extend the test suite.

## File Structure

- `test_cases.json` - Main test cases file containing all test scenarios
- `test_all.py` - Test runner that loads and executes tests from JSON
- `example_new_test.json` - Example showing how to add new test cases

## Test Categories

The JSON file contains three main categories:

1. **ml_parser_test_cases** - Tests for the ML parser functionality
2. **microservice_test_cases** - Tests for the FastAPI microservice
3. **real_world_scenarios** - Real-world log scenarios for comprehensive testing

## Running Tests

### Basic Usage
```bash
# Run ML parser tests only
python test_all.py --ml-only

# Run microservice tests only
python test_all.py --microservice

# Run all tests
python test_all.py --all

# Run specific test types
python test_all.py --real-world --performance --health
```

### Test Management
```bash
# List all available test cases
python test_all.py --list

# Add a new test case from JSON file
python test_all.py --add-test example_new_test.json
```

## Adding New Test Cases

### Method 1: Using JSON File
1. Create a JSON file with your test case (see `example_new_test.json`)
2. Run: `python test_all.py --add-test your_test.json`

### Method 2: Direct JSON Structure
```json
{
  "category": "ml_parser_test_cases",
  "name": "Your Test Name",
  "description": "Description of what this test covers",
  "lines": [
    "log line 1",
    "log line 2",
    "log line 3"
  ],
  "expect": [
    {"level": "INFO", "message_contains": "expected text"},
    {"message_contains": "another expected text"},
    {"level": "ERROR", "message_contains": "error message"}
  ]
}
```

### Method 3: Programmatically
```python
from test_all import add_test_case

new_test = {
    "name": "Custom Test",
    "description": "My custom test case",
    "lines": ["log line 1", "log line 2"],
    "expect": [{"message_contains": "expected"}]
}

add_test_case("ml_parser_test_cases", new_test)
```

## Test Case Structure

### ML Parser Test Cases
```json
{
  "name": "Test Name",
  "description": "Test description",
  "lines": ["log line 1", "log line 2"],
  "expect": [
    {"level": "INFO", "message_contains": "text"},
    {"message_contains": "text"}
  ]
}
```

### Microservice Test Cases
```json
{
  "name": "Test Name",
  "description": "Test description",
  "content": "log line 1\nlog line 2\nlog line 3",
  "expected_fields": ["timestamp", "level", "message"]
}
```

### Real World Scenarios
```json
{
  "name": "Scenario Name",
  "description": "Scenario description",
  "lines": ["log line 1", "log line 2"]
}
```

## Expectations Format

### Level Expectations
```json
{"level": "INFO"}  // Must match exact level
{"level": "ERROR"} // Must match exact level
```

### Message Expectations
```json
{"message_contains": "text"}  // Message must contain this text
```

### Combined Expectations
```json
{"level": "INFO", "message_contains": "text"}  // Both level and message
```

## Benefits of JSON-Based System

1. **Easy Maintenance** - Test cases are separate from code
2. **Extensibility** - Add new tests without modifying Python code
3. **Readability** - JSON format is human-readable
4. **Version Control** - Track test case changes in git
5. **Collaboration** - Multiple developers can add test cases
6. **Automation** - Easy to generate test cases programmatically

## Best Practices

1. **Descriptive Names** - Use clear, descriptive test names
2. **Good Descriptions** - Explain what each test covers
3. **Realistic Data** - Use realistic log examples
4. **Edge Cases** - Include edge cases and error scenarios
5. **Consistent Format** - Follow the established JSON structure
6. **Comprehensive Coverage** - Test different log formats and scenarios

## Example: Adding a New Log Format

If you want to add support for a new log format:

1. Create a test case with sample logs
2. Define expected parsing results
3. Add the test case to the JSON file
4. Run the test to verify parsing works
5. If parsing fails, enhance the ML parser
6. Re-run tests until all pass

This iterative approach ensures robust log parsing for any format! 