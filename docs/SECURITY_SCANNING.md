# Security Scanning Workflows

This document describes the security scanning workflows integrated into the CDS (Customer Data Service) repository.

## Overview

The repository includes three automated security scanning workflows to ensure code quality and security:

1. **FOSSA SCA Scan** - Software Composition Analysis for dependency vulnerabilities
2. **CodeQL Analysis** - Static code analysis for security vulnerabilities
3. **Trivy Security Scan** - Comprehensive security scanning for vulnerabilities, secrets, and misconfigurations

## Workflows

### 1. FOSSA SCA Scan

**File:** `.github/workflows/fossa-scan.yml`

**Purpose:** Scans project dependencies for known vulnerabilities, license compliance issues, and security risks.

**Triggers:**
- Push to `main` or `mvp` branches
- Pull requests to `main` or `mvp` branches
- Weekly schedule (Monday at 00:00 UTC)

**Requirements:**
- Requires `FOSSA_API_KEY` secret to be configured in repository settings
- To set up FOSSA:
  1. Sign up at [FOSSA](https://fossa.com/)
  2. Obtain API key from FOSSA dashboard
  3. Add the API key as `FOSSA_API_KEY` in repository secrets (Settings → Secrets and variables → Actions)

**What it scans:**
- Go module dependencies (go.mod, go.sum)
- Known CVEs in dependencies
- License compliance

### 2. CodeQL Analysis

**File:** `.github/workflows/codeql-analysis.yml`

**Purpose:** Performs static code analysis to identify security vulnerabilities, bugs, and code quality issues in the Go source code.

**Triggers:**
- Push to `main` or `mvp` branches
- Pull requests to `main` or `mvp` branches
- Weekly schedule (Monday at 02:00 UTC)

**Requirements:**
- No additional configuration required
- Uses GitHub's built-in CodeQL analysis
- Results appear in the Security tab under "Code scanning alerts"

**What it scans:**
- Go source code
- Security vulnerabilities (SQL injection, XSS, etc.)
- Code quality issues
- Common programming errors
- Uses both `security-extended` and `security-and-quality` query suites

**Permissions:**
- `actions: read` - Read workflow information
- `contents: read` - Read repository contents
- `security-events: write` - Upload security findings

### 3. Trivy Security Scan

**File:** `.github/workflows/trivy-scan.yml`

**Purpose:** Comprehensive security scanner for vulnerabilities, secrets, and configuration issues in the filesystem.

**Triggers:**
- Push to `main` or `mvp` branches
- Pull requests to `main` or `mvp` branches
- Weekly schedule (Monday at 04:00 UTC)

**Requirements:**
- No additional configuration required
- Results appear in the Security tab under "Code scanning alerts"

**What it scans:**
- Vulnerabilities in dependencies (CRITICAL, HIGH, MEDIUM severity)
- Hardcoded secrets in code
- Infrastructure as Code (IaC) misconfigurations
- Dockerfile security issues

**Permissions:**
- `contents: read` - Read repository contents
- `security-events: write` - Upload security findings

**Output:**
- SARIF format uploaded to GitHub Security tab
- Table format displayed in workflow logs

## Viewing Scan Results

### Security Tab
1. Navigate to repository → **Security** tab
2. Click on **Code scanning alerts** to view findings from CodeQL and Trivy
3. Filter by tool, severity, or status

### Workflow Logs
1. Navigate to repository → **Actions** tab
2. Select the specific workflow run
3. View detailed logs and scan output

### FOSSA Dashboard
1. Log in to [FOSSA dashboard](https://app.fossa.com/)
2. Find your project
3. View detailed dependency analysis, vulnerabilities, and license compliance

## Best Practices

1. **Review alerts regularly** - Check the Security tab for new findings
2. **Address critical issues first** - Prioritize CRITICAL and HIGH severity vulnerabilities
3. **Keep dependencies updated** - Regularly update Go modules to patch known vulnerabilities
4. **Fix before merging** - Address security findings before merging pull requests
5. **Weekly reviews** - All scans run weekly to catch new vulnerabilities

## Troubleshooting

### FOSSA Scan Fails
- **Issue:** Missing API key
- **Solution:** Ensure `FOSSA_API_KEY` is configured in repository secrets

### CodeQL Analysis Takes Too Long
- **Issue:** Large codebase
- **Solution:** This is normal; CodeQL performs deep analysis. Wait for completion or check logs for errors.

### Trivy Shows False Positives
- **Issue:** False positive vulnerability alerts
- **Solution:** Review the alert details in the Security tab and dismiss if confirmed as false positive with appropriate justification

## Maintenance

- **Update scan schedules** - Modify cron expressions in workflow files if needed
- **Adjust severity thresholds** - Change severity filters in Trivy scan if different levels are required
- **Update Go version** - Keep Go version in workflows aligned with project requirements

## Additional Resources

- [FOSSA Documentation](https://docs.fossa.com/)
- [CodeQL Documentation](https://codeql.github.com/docs/)
- [Trivy Documentation](https://aquasecurity.github.io/trivy/)
- [GitHub Code Scanning](https://docs.github.com/en/code-security/code-scanning)
