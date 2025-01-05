# Hello, fellow Space Monkey

> "You are not special. You’re not a beautiful and unique snowflake.
> You’re the same decaying organic matter as everything else.
> We’re all part of the same compost heap.
> We’re all singing, all dancing crap of the world."
>
> "You are not your job, you’re not how much money you have in the bank.
> You are not the car you drive.
> You’re not the contents of your wallet.
> You are not your fucking khakis.
> You are all singing, all dancing crap of the world."
>
> _Fight Club by Chuck Palahniuk_

Things that we swear by the [rubber duck](https://en.wikipedia.org/wiki/Rubber_duck_debugging) that we would follow so long as it's sensible:

- [Git hygiene](https://cbea.ms/git-commit/)
- [Effective Go](https://go.dev/doc/effective_go)
- [Code of Conduct](CODE_OF_CONDUCT.md)
- [Pull Request Guidelines](PULL_REQUEST_GUIDELINES.md)

## Before contributing code

We welcome code patches, but to ensure smooth coordination, please discuss any significant changes before starting work. Signal your intention to contribute in the issue tracker by filing a new issue or claiming an existing one.

## Check the issue tracker

Whether you have a specific contribution in mind or are looking for ideas, the issue tracker is the first place to go. Issues are categorized to manage workflow.

Common workflow labels:

- **good first issue**: Good for newcomers.
- **Essential**: It is understood that this can't be overlooked, and demands attention.
- **Needs Investigation**: The issue requires analysis to understand the root cause.
- **Needs Decision**: The issue is understood, but the best solution is undecided. Wait for a decision before writing code. Feel free to "ping" maintainers if no decision has been made after some time.
- **Needs Fix**: The issue is understood and ready for code to be written.

## Open an issue for any new problem

Except for trivial changes, all contributions should be linked to an existing issue. Open one and discuss your plans. This process validates the design, prevents duplication, ensures alignment with project goals, and checks the design before code is written. **The code review tool is not for high-level discussions.**

## [The seven rules of a great Git commit message](https://cbea.ms/git-commit/)

1. Separate subject from body with a blank line
2. Limit the subject line to 50 characters
3. Capitalize the subject line
4. Do not end the subject line with a period
5. Use the imperative mood in the subject line
6. Wrap the body at 72 characters
7. Use the body to explain what and why vs. how

Example:

```git
Summarize changes in around 50 characters or less

More detailed explanatory text, if necessary. Wrap it to about 72
characters or so. In some contexts, the first line is treated as the
subject of the commit and the rest of the text as the body. The
blank line separating the summary from the body is critical (unless
you omit the body entirely); various tools like `log`, `shortlog`
and `rebase` can get confused if you run the two together.

Explain the problem that this commit is solving. Focus on why you
are making this change as opposed to how (the code explains that).
Are there side effects or other unintuitive consequences of this
change? Here's the place to explain them.

Further paragraphs come after blank lines.

- Bullet points are okay, too

- Typically a hyphen or asterisk is used for the bullet, preceded
    by a single space, with blank lines in between, but conventions
    vary here

If you use an issue tracker, put references to them at the bottom,
like this:

Resolves: #123 // closes the issue
Updates: #123 // references the issue without closing
See also: #456, #789
```
