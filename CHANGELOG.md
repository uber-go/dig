# Changelog

## v1.0.0-rc2 (unreleased)

- So that we can introduce new functionality after a 1.0 release, add a
  variadic options parameter to all public APIs.

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
