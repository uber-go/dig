# Changelog


## v0.4 (unreleased)
- Add support form `map`, `slice` and `array` to dig

## v0.3 (2 May 2017)

- Rename `RegisterAll` and `MustRegisterAll` to `ProvideAll` and
  `MustProvideAll`.
- Add functionality to `Provide` to support constructor with `n` return
  objects to be resolved into the `dig.Graph`
- Add `Invoke` function to invoke provided function and insert return
  objects into the `dig.Graph`

## v0.2 (27 Mar 2017)

- Rename `Register` to `Provide` for clarity and to recude clash with other
  Register functions.
- Rename `dig.Graph` to `dig.Container`.
- Remove the package-level functions and the `DefaultGraph`.

## v0.1 (23 Mar 2017)

Initial release.
