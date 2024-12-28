# Proz: Effortless Project Directory Management

Proz is a free and open source command line tool to help developers manage
their projects.

## Features

- Add Projects: Save project directories with custom names and tags for easy organization.

- List Projects: View your projects in a list.

- Open Projects: Launch projects using a specified executable
(e.g., code, vim, notepad) or cd into them directly.

## Usage

### Add a project

```bash
proz add proz-code
Enter the command/binary to open this project (e.g., code, nvim):
code
Enter arguments for the command (space-separated, or press Enter for none):
Project 'proz-code' added successfully!
```

### Open a project

```bash
proz # 'proz list' also workd
Stored projects
[1] proz (Path: /home/njayman/Development/proz, Tags: [proz, go, nvim])
[2] proz-code (Path: /home/njayman/Development/proz, Tags: [proz, go, vscode])
Select a project to open by number: 2
Opening project 'proz-code' with command: code []

```

## Planned Features

- Tag-Based Filtering: Filter projects by tags for faster selection.
- TUI Mode: Add a text-based user interface (TUI) for enhanced navigation.

## Contributing

Contributions are welcome! Here's how you can help:

- Fork this repository.
- Create a feature branch (git checkout -b feature-name).
- Commit your changes (git commit -m "Add new feature").
- Push the branch (git push origin feature-name).
- Open a pull request.

## Acknowledgments

Special thanks to the open-source community for inspiration and support
in building this tool.

Feel free to suggest edits, enhancements, or additional features in the issues section!
