# Proz: Effortless Project Directory Management

Proz is a free and open source command line tool to help developers manage
their projects.

## Features

- Add Projects: Save project directories with custom names and tags for easy organization.
- List Projects: View your projects in a list.
- Delete Projects: Remove projects from your list.
- Open Projects: Launch projects using a specified executable
(e.g., code, vim, notepad) or cd into them directly.
- Tag-Based Filtering: Filter projects by tags when listing.
- Shell Completion: Tab completion for bash, zsh, fish, and PowerShell.

## Usage

### Add a project

```bash
proz add proz-code
Enter the command/binary to open this project (e.g., code, nvim):
code
Enter arguments for the command (space-separated, or press Enter for none):
Project 'proz-code' added successfully!
```

### List projects

```bash
proz list
```

### Filter by tags

```bash
proz list --tags go,cli
```

### Delete a project

```bash
proz delete
# or
proz delete proz-code
```

### Open a project

```bash
proz
Stored projects
[1] proz (Path: /home/user/projects/proz, Tags: [proz, go])
Select a project to open by number: 1
```

### Generate shell completion

```bash
source <(proz completion bash)    # bash
source <(proz completion zsh)     # zsh
proz completion fish > ~/.config/fish/completions/proz.fish  # fish
```

## Planned Features

- TUI Mode: Add a text-based user interface (TUI) for enhanced navigation.

## Contributing

Contributions are welcome! Here's how you can help:

- Fork this repository.
- Create a feature branch (git checkout -b feature-name).
- Commit your changes (git commit -m "Add new feature").
- Push the branch (git push origin feature-name).
- Open a pull request.

## License

GNU General Public License v3. See [LICENSE](LICENSE).
