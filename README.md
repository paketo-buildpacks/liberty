# `gcr.io/paketo-buildpacks/open-liberty`

The Paketo Open Liberty Buildpack is a Cloud Native Buildpack that contributes Open Liberty for Java EE support.

## Behavior

This buildpack will participate if all the following conditions are met

* `Main-Class` is NOT defined in the mainfest
* `<APPLICATION_ROOT>/META-INF/application.xml` or `<APPLICATION_ROOT>/WEB-INF` exist

The buildpack will do the following:

* Requests that a JRE be installed
* Contribute an Open Liberty runtime and create a server called `defaultServer`
* Contributes `web` process type
* At launch time, symlink `<APPLICATION_ROOT>` to `defaultServer/apps/<APPLICATION_ROOT_BASENAME>`.

The buildpack will support all available profiles of the two most recent versions of the Open Liberty runtime. Because the Open Liberty versioning scheme is not conformant to semantic versioning, an Open Liberty version like `21.0.0.11` is defined here as `21.0.0`, and should be referenced as such.

## Configuration

| Environment Variable            | Description                                                                                                                                                                                                                                     |
| ------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `$BP_OPENLIBERTY_INSTALL_TYPE`              | [Install type](#install-types) of Liberty. Valid options: `ol`, `wlp`, `ol-stack`, `wlp-stack`. Defaults to `ol`.                                                                                                                               |
| `$BP_OPENLIBERTY_VERSION`       | The version of Open Liberty to install. Defaults to the latest version of the runtime.                                                                                                                                                          |
| `$BP_OPENLIBERTY_PROFILE`       | The Open Liberty profile to use. Defaults to `full`.                                                                                                                                                                                            |
| `$BP_OPENLIBERTY_CONTEXT_ROOT`  | If the [server.xml](#bindings) does not have an [application](https://openliberty.io/docs/21.0.0.12/reference/config/application.html) named `app` defined, then the buildpack will generate one and use this value as the context root. Defaults to the value of `/`. |
| `$BPL_OPENLIBERTY_LOG_LEVEL`    | Sets the [logging](https://openliberty.io/docs/21.0.0.11/log-trace-configuration.html#configuaration) level. If not set, attempts to get the buildpack's log level. If unable, defaults to `INFO`                                               |

### Default Configurations that Vary from Open Liberty's Default

By default, the Open Liberty buildpack will log in `json` format. This will aid in log ingestion. Due to design decisions from the Open Liberty team, setting this format to any other value will prevent all log types from being sent to `stdout` and will instead go to `messages.log`. In addition, the log sources that will go to stdout are `message,trace,accessLog,ffdc,audit`.

All of these defaults can be overridden by setting the appropriate properties found in Open Liberty's [documentation](https://openliberty.io/docs/21.0.0.11/log-trace-configuration.html). They can be set as environment variables, or in [`bootstrap.properties`](#bindings).

## Install Types

There are four different installation types that can be configured:
* `ol`: This will download an Open Liberty runtime and use it when deploying the container.
* `wlp`: This will download a WebSphere Liberty runtime and use it when deploying the container.
* `ol-stack`: This will use an Open Liberty runtime provided in the stack run image. Requires an Open Liberty builder.
* `wlp-stack`: This will use a WebSphere Liberty runtime provided in the stack run image. Requires a WebSphere Liberty builder.

## Bindings

The buildpack accepts the following bindings:

### Type: `open-liberty`

| Key                    | Value             | Description                                                                                                                                                                   |
| ---------------------- | ----------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `server.xml`           | `<file-contents>` | This file will replace the `defaultServer`'s `server.xml` and is not subject to any post-processing; therefore, any variable references therein must be resolvable. Optional. |
| `bootstrap.properties` | `<file-contents>` | This file will replace the `defaultServer`'s `bootstrap.properties`. This is one place to define variables used by `server.xml`. Optional.                                    |

### Type: `dependency-mapping`

| Key                   | Value   | Description                                                                                       |
| --------------------- | ------- | ------------------------------------------------------------------------------------------------- |
| `<dependency-digest>` | `<uri>` | If needed, the buildpack will fetch the dependency with digest `<dependency-digest>` from `<uri>` |

## License

This buildpack is released under version 2.0 of the [Apache License][a].

[a]: http://www.apache.org/licenses/LICENSE-2.0

