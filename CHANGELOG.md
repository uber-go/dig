# Changelog

## v1.0.0-rc3 (unreleased)

- `Provide` and `Invoke` will now error out if `dig.In` or `dig.Out` structs
  contain private fields. Previously these fields were ignored which often led
  to confusion.

## v1.0.0-rc2 (21 Jul 2017)

- Added variadic options to all public APIS so that new functionality can be
  introduced post v1.0.0 without introducing breaking changes.
- Functions with variadic arguments can now be passed to `dig.Provide` and
  `dig.Invoke`. Previously this caused an error, whereas now the args will be ignored.
- Exported `dig.IsIn` and `dig.IsOut` so that consuming libraries can check if
  a params or return struct embeds the `dig.In` and `dig.Out` types, respectively.

## v1.0.0-rc1 (21 Jun 2017)

- First release candidate.

## v0.5.0 (19 Jun 2017)

- `dig.In` and `dig.Out` now support named instances, i.e.:

  ```go
  type param struct {
    dig.In

    DB1 DB.Connection `name:"primary"`
    DB2 DB.Connection `name:"secondary"`
  }
  ```

- Structs compatible with `dig.In` and `dig.Out` may now be generated using
  `reflect.StructOf`.

## v0.4.0 (12 Jun 2017)

- **[Breaking]** Remove `Must*` funcs to greatly reduce API surface area.
- **[Breaking]** Restrict the API surface to only `Provide` and `Invoke`.
- Providing constructors with common returned types results in an error.
- **[Breaking]** Update `Provide` method to accept variadic arguments.
- Add `dig.In` embeddable type for advanced use-cases of specifying dependencies.
- Add `dig.Out` embeddable type for advanced use-cases of constructors
  inserting types in the container.
- Add support for optional parameters through `optional:"true"` tag on `dig.In` objects.
- Add support for value types and many built-ins (maps, slices, channels).

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
