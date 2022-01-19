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

| Environment Variable               | Description                                                                                                                                                                                                                                                         |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `$BP_OPENLIBERTY_INSTALL_TYPE`     | [Install type](#install-types) of Liberty. Valid options: `ol` and `none`. Defaults to `ol`.                                                                                                                                                                        |
| `$BP_OPENLIBERTY_VERSION`          | The version of Open Liberty to install. Defaults to the latest version of the runtime.                                                                                                                                                                              |
| `$BP_OPENLIBERTY_PROFILE`          | The Open Liberty profile to use. Defaults to `full`.                                                                                                                                                                                                                |
| `$BP_OPENLIBERTY_CONTEXT_ROOT`     | If the [server.xml](#bindings) does not have an [application](https://openliberty.io/docs/latest/reference/config/application.html) named `app` defined, then the buildpack will generate one and use this value as the context root. Defaults to the value of `/`. |
| `$BPL_OPENLIBERTY_LOG_LEVEL`       | Sets the [logging](https://openliberty.io/docs/21.0.0.11/log-trace-configuration.html#configuaration) level. If not set, attempts to get the buildpack's log level. If unable, defaults to `INFO`                                                                   |
| `$BP_OPENLIBERTY_EXT_CONF_SHA256`  | The SHA256 hash of the external configuration package.                                                                                                                                                                                                              |
| `$BP_OPENLIBERTY_EXT_CONF_STRIP`   | The number of directory levels to strip from the external configuration package. Defaults to 0.                                                                                                                                                                     |
| `$BP_OPENLIBERTY_EXT_CONF_URI`     | The download URI of the external configuration package.                                                                                                                                                                                                             | 
| `$BP_OPENLIBERTY_EXT_CONF_VERSION` | The version of the external configuration package.                                                                                                                                                                                                                  | 

### Default Configurations that Vary from Open Liberty's Default

By default, the Open Liberty buildpack will log in `json` format. This will aid in log ingestion. Due to design decisions from the Open Liberty team, setting this format to any other value will prevent all log types from being sent to `stdout` and will instead go to `messages.log`. In addition, the log sources that will go to stdout are `message,trace,accessLog,ffdc,audit`.

All of these defaults can be overridden by setting the appropriate properties found in Open Liberty's [documentation](https://openliberty.io/docs/21.0.0.11/log-trace-configuration.html). They can be set as environment variables, or in [`bootstrap.properties`](#bindings).

## Install Types

The different installation types that can be configured are:
* `ol`: This will download an Open Liberty runtime and use it when deploying the container.
* `none`: This will use the Liberty runtime provided in the stack run image. Requires a custom builder.

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

## Using Custom Features

Custom features can be configured on the server by supplying an external configuration package containing the feature
JAR and manifest along with a feature descriptor.

### Feature Manifest

The feature manifest is a TOML file called `features.toml` containing a list of `features` that should be installed on
the server.

A feature has the properties:

* `name`: Name of the feature to enable. Use the symbolic name of the feature that you would use when enabling the feature in the `server.xml`.
* `uri`: URI of where to find the feature. The `file` scheme is the only supported scheme at the moment.
* `version`: Version of the feature.
* `dependencies`: List of features that the custom feature depends on, if any.

### Example Feature Manifest

This example shows how to configure a feature called `dummyCache` that has a dependency on the `distributedMap-1.0`
feature.

First create the feature descriptor `features.toml` with the following content:

```toml
[[features]]
  name = "dummyCache"
  uri = "file:/features/cache.dummy_1.0.0.jar"
  version = "1.0.0"
  dependencies = ["distributedMap-1.0"]
```

Using this feature description, the Open Liberty buildpack will look for the feature JAR in the external configuration
package at the path `features/cache.dummy_1.0.0.jar`. The buildpack also assumes that the feature manifest file will be
at the path `features/cache.dummy_1.0.0.mf`.

After creating the feature descriptor, tar and gzip the `feature.toml` and `features` directory so that it has the
contents similar to the following:

```console
$ tar tzf liberty-conf.tar.gz
./
./features/
./features.toml
./features/cache.dummy_1.0.0.mf
./features/cache.dummy_1.0.0.jar
```

The external configuration package can be provided to the build by providing the `BP_OPENLIBERTY_EXT_CONF_*`
environment variables to the build. For example, if the external configuration is hosted on a web server, you can use:

```console
pack build --path myapp --env BP_OPENLIBERTY_EXT_CONF_URI=https://example.com/liberty-conf.tar.gz --env BP_OPENLIBERTY_EXT_CONF_VERSION=1.0.0 --env BP_OPENLIBERTY_EXT_CONF_SHA256=953e665e4126b75fecb375c88c51a1ddcf4d12474d43576323862d422e625517 myapp
```

If you'd like to provide the configuration as a file, you can do so by mounting the external configuration in the
container and then providing the path to the external configuration like so:

```console
pack build --path myapp --env BP_OPENLIBERTY_EXT_CONF_URI=file:///path/to/conf/liberty-conf.tar.gz --env BP_OPENLIBERTY_EXT_CONF_VERSION=1.0.0 --env BP_OPENLIBERTY_EXT_CONF_SHA256=953e665e4126b75fecb375c88c51a1ddcf4d12474d43576323862d422e625517 myapp
```

## License

This buildpack is released under version 2.0 of the [Apache License][a].

[a]: http://www.apache.org/licenses/LICENSE-2.0

