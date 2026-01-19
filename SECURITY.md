# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue, please report it responsibly.

### How to Report

**Preferred Method: GitHub Security Advisories**

1. Go to the [Security Advisories](https://github.com/sebrandon1/grab/security/advisories) page
2. Click "New draft security advisory"
3. Fill out the vulnerability details

**Alternative: Email**

If you prefer email, contact the maintainers directly through their GitHub profiles.

### What to Include

When reporting a vulnerability, please include:

- **Description**: A clear description of the vulnerability
- **Impact**: What an attacker could achieve by exploiting it
- **Reproduction Steps**: Step-by-step instructions to reproduce the issue
- **Affected Versions**: Which versions are impacted
- **Suggested Fix**: If you have ideas for remediation (optional)

### Response Timeline

- **Initial Response**: Within 48 hours of receiving the report
- **Status Update**: Within 7 days with an assessment
- **Resolution**: Target fix within 30 days for confirmed vulnerabilities

### What to Expect

1. We will acknowledge receipt of your report
2. We will investigate and validate the vulnerability
3. We will work on a fix and coordinate disclosure timing with you
4. We will credit you in the security advisory (unless you prefer anonymity)

### Scope

This security policy applies to:

- The `grab` CLI tool
- The `grab/lib` Go library
- Official container images (if any)

### Out of Scope

- Vulnerabilities in dependencies (please report to the respective projects)
- Issues that require physical access to the machine
- Social engineering attacks

## Security Best Practices for Users

When using `grab`:

- Verify file hashes after downloading sensitive files
- Download files from trusted sources only
- Keep the tool updated to the latest version
