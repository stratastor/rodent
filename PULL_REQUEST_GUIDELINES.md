# Pull Request Guidelines for Rodent

Thank you for your interest in contributing to Rodent! To ensure a smooth and effective review process, please follow these guidelines when submitting a pull request (PR).

## Forking Model Workflow

1. **Fork the Repository**: Create your own fork of the Rodent repository on GitHub.
2. **Clone Your Fork**:

    ```sh
    git clone https://github.com/stratastor/rodent.git
    cd rodent
    ```

3. **Add Upstream Remote**: Add the original repository as a remote named `upstream`.

    ```sh
    git remote add upstream https://github.com/stratastor/rodent.git
    ```

4. **Fetch Upstream Changes**: Fetch the latest changes from the upstream repository.

    ```sh
    git fetch upstream
    ```

5. **Rebase Your Fork**: Rebase your fork's main branch with the upstream main branch.

    ```sh
    git checkout main
    git pull --rebase upstream main
    ```

6. **Create a Branch**: Create a new branch for your feature or bugfix.

    ```sh
    git checkout -b feature/kickarse-api-module
    ```

7. **Make Your Changes**: Write your code and tests.
8. **Run Tests**: Ensure all tests pass before committing.

    ```sh
    ubuntu@staging:~/rodent$ cd pkg/zfs && sudo go test -v ./...
    ```

9. **Commit Your Changes**: Commit your changes with a descriptive message.

    ```sh
    git add .
    git commit -a 
    ```

10. **Push to Your Fork**: Push your branch to your fork on GitHub.

    ```sh
    git push origin feature/kickarse-api-module
    ```

11. **Create a Pull Request**: Go to the original Rodent repository on GitHub and create a pull request from your fork.

## Pull Request Checklist

Before submitting your pull request, please ensure the following:

1. **Descriptive Title and Description**: Provide a clear and concise title and description for your pull request. Explain the purpose and motivation behind your changes.
2. **Reference Issues**: If your pull request addresses any open issues, reference them in the description using the format `Fixes #issue_number` or `Updates/Related to #issue_number`.
3. **Code Style**: Ensure your code adheres to the project's coding standards and style guidelines.
4. **Tests**: Add tests for your changes and ensure all existing tests pass.
5. **Documentation**: Update any relevant documentation to reflect your changes.
6. **Review Your Changes**: Double-check your code for any errors or improvements before submitting.

## During the Review Process

1. **Be Responsive**: Be available to respond to any questions or feedback from the reviewers.
2. **Make Necessary Changes**: If requested, make the necessary changes and update your pull request.
3. **Keep Commits Clean**: Squash or rebase your commits if necessary to keep the commit history clean and meaningful.

## Additional Resources

- [Go Documentation](https://golang.org/doc/)
- [Effective Go](https://golang.org/doc/effective_go.html)

Thank you for contributing to Rodent! Your efforts are greatly appreciated.
