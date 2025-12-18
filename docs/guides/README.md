# User Guides

This section provides comprehensive guides for using Bib.

## In This Section

| Document | Description |
|----------|-------------|
| [CLI Reference](cli-reference.md) | Complete command reference for `bib` |
| [Jobs & Tasks](jobs-tasks.md) | Creating and running data processing jobs |
| [Clustering](clustering.md) | Setting up high-availability Raft clusters |

## Quick Reference

### Common Commands

```bash
# Setup and configuration
bib setup              # Configure CLI
bib setup --daemon     # Configure daemon
bib config show        # View configuration

# Data management
bib topic list         # List topics
bib dataset list       # List datasets
bib catalog query      # Search the catalog

# Job management
bib job submit         # Submit a job
bib job status <id>    # Check job status
bib task list          # List available tasks

# Cluster management
bib cluster status     # Check cluster status
bib cluster members    # List cluster members
```

### Output Formats

All commands support multiple output formats:

```bash
bib topic list -o json   # JSON output
bib topic list -o yaml   # YAML output
bib topic list -o table  # Table with borders
```

---

[← Back to Documentation](../README.md)

