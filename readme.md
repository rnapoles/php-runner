# PHP Runner

A lightweight Go-based PHP tool that automatically switches between different PHP versions based on project-specific configuration files.

## Overview

PHP Runner is a simple command-line tool that allows you work with multiple PHP installations and automatically use the correct PHP version for each project. Similar to tools like `rbenv` for Ruby or `nvm` for Node.js, it reads a `.php-version` file in your project directory to determine which PHP executable to use.

## Features

- **Automatic Version Detection**: Reads `.php-version` files to determine the correct PHP version for each project
- **Fallback Logic**: If no `.php-version` file exists, it detects your current PHP version and creates the file automatically
- **Multiple PHP Support**: Configure multiple PHP installations through a simple configuration file
- **Transparent Execution**: Passes all arguments directly to the selected PHP executable
- **Directory Traversal**: Searches for `.php-version` files in current and parent directories
- **Executable Verification**: Validates that all configured PHP executables exist on disk

## How It Works

1. **Configuration**: Define your PHP versions and their paths in `php-runner.yaml`
2. **Project Setup**: Create a `.php-version` file in your project root with the desired version (e.g., `8.2`)
3. **Execution**: Run `php-runner` instead of `php` - it automatically uses the correct PHP version
4. **Auto-Creation**: If no `.php-version` exists, it detects your current PHP and creates the file

## Configuration Example

Create `php-runner.yaml` in the same directory as the executable or in your home dir:

```yaml
5.6: C:\dev\php\5.6.8\php.exe
7.0: C:\dev\php\7.0.33\php.exe
7.1: C:\dev\php\7.1.33\php.exe
7.2: C:\dev\php\7.2.34\php.exe
7.3: C:\dev\php\7.3.15\php.exe
7.4: C:\dev\php\7.4.3\php.exe
8.0: C:\dev\php\8.0.2\php.exe
8.1: C:\dev\php\8.1.0\php.exe
8.2: C:\dev\php\8.2.0\php.exe
8.3: C:\dev\php\8.3\php.exe
8.4: C:\dev\php\8.4\php.exe
```

## Usage

```bash
# Instead of: php script.php
# Use: php-runner script.php

# All PHP commands work transparently
php-runner --version
php-runner composer.phar install
php-runner artisan serve
```

## Installation

1. Build the executable: `go build -o php-runner.exe`
2. Place `php-runner.exe` in your desired location (e.g., `C:\dev\`)
3. Create `php-runner.yaml` in the same directory
4. Add the directory to your system PATH
5. Use `php-runner` instead of `php` in your projects

## Requirements

- Go 1.16+ (for building)
- Multiple PHP installations on your system
- Windows, Linux, or macOS

## Benefits

- **Project Isolation**: Each project can use its own PHP version
- **Team Consistency**: Ensures all team members use the same PHP version
- **Legacy Support**: Easily work with older projects requiring specific PHP versions
- **Zero Configuration**: Automatically detects and configures versions when possible
- **Lightweight**: No external dependencies, single executable file