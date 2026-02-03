# Label Management for readme-runner

This document describes the label system used in the readme-runner repository.

## Label Categories

### Core Feature Areas
Labels that identify which part of the tool is affected:
- **cli** - Command-line interface functionality
- **automation** - Automated installation and execution features
- **readme-parsing** - README.md parsing and analysis
- **workspace** - Workspace management and temporary directories
- **git-integration** - Git repository cloning and fetching

### Technology Stack
Labels for technology-specific issues:
- **golang** - Go language implementation

### Issue Types
Standard issue classification:
- **bug** - Something isn't working correctly
- **enhancement** - New feature or request
- **documentation** - Documentation improvements
- **security** - Security-related issues
- **performance** - Performance optimizations

### Priority Levels
Used to indicate urgency:
- **priority:high** - High priority issue
- **priority:medium** - Medium priority issue
- **priority:low** - Low priority issue

### Development Status
Community and workflow labels:
- **good first issue** - Good for newcomers
- **help wanted** - Community help needed
- **wontfix** - Will not be addressed
- **duplicate** - Duplicate of another issue

### Feature Phases
Based on the 7-phase execution model in the code:
- **phase:fetch** - Phase 1: Fetch/Workspace
- **phase:scan** - Phase 2: Scan
- **phase:plan** - Phase 3: Plan (AI)
- **phase:validate** - Phase 4: Validate/Normalize
- **phase:prerequisites** - Phase 5: Prerequisites
- **phase:execute** - Phase 6: Execute
- **phase:cleanup** - Phase 7: Post-run/Cleanup

## Applying Labels

### Manual Application
Labels can be applied manually through the GitHub web interface when creating or editing issues and pull requests.

### Automated Sync
To automatically sync these labels with your GitHub repository, you can use tools like:

1. **github-label-sync** (Node.js):
   ```bash
   npx github-label-sync --access-token $GITHUB_TOKEN sony-level/readme-runner .github/labels.yml
   ```

2. **labeler GitHub Action** - Add to `.github/workflows/labels.yml`:
   ```yaml
   name: Sync Labels
   on:
     push:
       branches: [main]
       paths:
         - '.github/labels.yml'
   
   jobs:
     sync:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v3
         - uses: micnncim/action-label-syncer@v1
           env:
             GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
           with:
             manifest: .github/labels.yml
   ```

3. **GitHub CLI**:
   ```bash
   # Install GitHub CLI first
   gh label create "cli" --color "0052CC" --description "Related to command-line interface functionality"
   ```

## Label Usage Guidelines

### When Creating Issues
1. Always add at least one **issue type** label (bug, enhancement, documentation, etc.)
2. Add relevant **feature area** labels to help categorize the issue
3. For phase-specific issues, add the appropriate **phase** label
4. Consider adding a **priority** label for planning purposes

### For Pull Requests
1. Link PRs to related issues to inherit labels automatically
2. Add **phase** labels when implementing specific functionality
3. Use **documentation** label when updating docs

## Examples

### Bug Report
- Labels: `bug`, `cli`, `priority:high`
- Description: "Error when parsing multi-line shell commands in README"

### Feature Request
- Labels: `enhancement`, `phase:plan`, `help wanted`
- Description: "Add support for AI-based installation plan generation"

### Documentation Update
- Labels: `documentation`, `good first issue`
- Description: "Add examples for using --dry-run flag"

## Contributing

When contributing to readme-runner, please use these labels to help maintainers understand and prioritize your contributions. If you think a new label would be helpful, please open an issue to discuss it.
