#!/bin/bash
# NOVA Development Utilities
# Usage: source scripts/dev-utils.sh
# Or run directly: ./scripts/dev-utils.sh <command>

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

#=============================================================================
# CURRENT BRANCH STRUCTURE
#=============================================================================

# Show project structure (depth-limited)
nova-tree() {
    local depth=${1:-3}
    echo -e "${BLUE}=== Project Structure (depth=$depth) ===${NC}"
    tree -L "$depth" -I 'vendor|node_modules|.git|bin' --dirsfirst
}

# List all Go files
nova-go-files() {
    echo -e "${BLUE}=== Go Files ===${NC}"
    find . -name "*.go" -not -path "./vendor/*" | sort
}

# List all functions in a package
nova-funcs() {
    local pkg=${1:-"internal/core/deployment"}
    echo -e "${BLUE}=== Functions in $pkg ===${NC}"
    grep -rn "^func " "$pkg" --include="*.go" | sed 's/:func /: func /'
}

# List YAML resources
nova-resources() {
    echo -e "${BLUE}=== YAML Resources ===${NC}"
    find resources -name "*.yaml" | sort
}

#=============================================================================
# MAIN BRANCH STRUCTURE (Reference Scripts)
#=============================================================================

# List all files on main branch
main-files() {
    echo -e "${BLUE}=== Files on main branch ===${NC}"
    git ls-tree -r main --name-only
}

# List specific directory on main
main-ls() {
    local dir=${1:-"01-lab-setup"}
    echo -e "${BLUE}=== main:$dir ===${NC}"
    git ls-tree -r "main:$dir" --name-only 2>/dev/null || git ls-tree main "$dir" --name-only
}

# Show file content from main
main-cat() {
    local file=$1
    if [ -z "$file" ]; then
        echo -e "${RED}Usage: main-cat <path>${NC}"
        return 1
    fi
    echo -e "${BLUE}=== main:$file ===${NC}"
    git show "main:$file"
}

# List tier scripts on main
main-tier() {
    local tier=${1:-1}
    echo -e "${BLUE}=== Tier $tier scripts on main ===${NC}"
    git ls-tree -r "main:01-lab-setup/0${tier}-tier${tier}-setup" --name-only 2>/dev/null || \
    git ls-tree -r "main:01-lab-setup/00-k8s-setup" --name-only
}

#=============================================================================
# COMPARISON UTILITIES
#=============================================================================

# Diff a file between main and current branch
nova-diff() {
    local file=$1
    if [ -z "$file" ]; then
        echo -e "${RED}Usage: nova-diff <path>${NC}"
        return 1
    fi
    echo -e "${BLUE}=== Diff: main vs current for $file ===${NC}"
    git diff main -- "$file"
}

# List all changed files vs main
nova-changes() {
    echo -e "${BLUE}=== Changed files vs main ===${NC}"
    git diff main --name-status
}

# Compare resource directories between main and current
nova-compare-resources() {
    local main_dir=${1:-"01-lab-setup/02-tier2-setup/resources"}
    local current_dir=${2:-"resources/core/deployment/tier2"}

    echo -e "${BLUE}=== Comparing resources ===${NC}"
    echo -e "${YELLOW}Main ($main_dir):${NC}"
    git ls-tree -r "main:$main_dir" --name-only 2>/dev/null | sort
    echo ""
    echo -e "${YELLOW}Current ($current_dir):${NC}"
    find "$current_dir" -name "*.yaml" | sed "s|$current_dir/||" | sort
}

# Find files in main that don't exist in current branch
nova-missing() {
    local main_dir=${1:-"01-lab-setup/02-tier2-setup/resources"}
    local current_dir=${2:-"resources/core/deployment/tier2"}

    echo -e "${BLUE}=== Files in main but missing in current ===${NC}"
    git ls-tree -r "main:$main_dir" --name-only 2>/dev/null | while read -r f; do
        # Translate k8s-lab naming to nova
        local translated=$(echo "$f" | sed 's/k8s-lab/nova/g')
        local current_file="$current_dir/$translated"
        if [ ! -f "$current_file" ]; then
            echo -e "${RED}Missing:${NC} $f -> $current_file"
        fi
    done
}

#=============================================================================
# QUICK HELPERS
#=============================================================================

# Show help
nova-help() {
    echo -e "${GREEN}NOVA Development Utilities${NC}"
    echo ""
    echo -e "${YELLOW}Current Branch:${NC}"
    echo "  nova-tree [depth]      - Show project structure"
    echo "  nova-go-files          - List all Go files"
    echo "  nova-funcs [pkg]       - List functions in package"
    echo "  nova-resources         - List YAML resources"
    echo ""
    echo -e "${YELLOW}Main Branch (Reference):${NC}"
    echo "  main-files             - List all files on main"
    echo "  main-ls [dir]          - List directory on main"
    echo "  main-cat <file>        - Show file from main"
    echo "  main-tier [1|2|3]      - List tier scripts"
    echo ""
    echo -e "${YELLOW}Comparison:${NC}"
    echo "  nova-diff <file>       - Diff file vs main"
    echo "  nova-changes           - List all changed files"
    echo "  nova-compare-resources - Compare resource dirs"
    echo "  nova-missing           - Find missing files"
    echo ""
    echo -e "${YELLOW}Note:${NC} k8s-lab -> nova translation is automatic"
}

# If script is run directly (not sourced), execute the command
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    if [ $# -eq 0 ]; then
        nova-help
    else
        "$@"
    fi
fi
