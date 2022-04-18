# `gcr.io/paketo-buildpacks/liberty`

The Paketo Liberty Buildpack is a Cloud-Native Buildpack that contributes Open Liberty or WebSphere Liberty for Java EE support.

## Behavior

The buildpack will participate when building any of the following:

* A Java web application from source or compiled artifact
* A packaged Liberty server created using the `server package` [command](https://openliberty.io/docs/latest/reference/command/server-package.html)
* A Liberty root directory

When building a web application, this buildpack will participate if all the following conditions are met:

* `$BP_JAVA_APP_SERVER` is `liberty` or if `$BP_JAVA_APP_SERVER` is unset or empty and this is the first buildpack to provide a Java application server.
* `Main-Class` is NOT defined in the manifest
* `<APPLICATION_ROOT>/META-INF/application.xml` or `<APPLICATION_ROOT>/WEB-INF` exist

When building from a packaged Liberty server or from a Liberty root directory, the buildpack will participate if all the
following conditions are met:

* `<APPLICATION_ROOT>/wlp/usr/servers/$BP_LIBERTY_SERVER_NAME/server.xml` exists
* At least one application is installed at either `<APPLICATION_ROOT>/wlp/usr/servers/$BP_LIBERTY_SERVER_NAME/apps` or
  `<APPLICATION_ROOT>/wlp/usr/servers/$BP_LIBERTY_SERVER_NAME/dropins`

The buildpack will do the following:

* Requests that a JRE be installed
* Contribute an Open Liberty or WebSphere Liberty runtime and create a server called `defaultServer`
* Contributes `web` process type
* If a web application was built, it will be symlink `<APPLICATION_ROOT>` to `defaultServer/apps/<APPLICATION_ROOT_BASENAME>`
  at launch time
* If a Liberty server was built, it will symlink `<APPLICATION_ROOT>` to `<RUNTIME_ROOT>/usr`

The buildpack will support all available profiles of the most recent versions of the Open Liberty runtime. Because the Liberty versioning scheme is not conformant to semantic versioning, an Liberty version like `22.0.0.2` is defined here as `22.0.2`, and should be referenced as such.

## Configuration

| Environment Variable           | Description                                                                                                                                                                                                                                                         |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `$BP_JAVA_APP_SERVER`          | The application server to use. It defaults to `` (empty string) which means that order dictates which Java application server is installed. The first Java application server buildpack to run will be picked.                                                      |
| `$BP_LIBERTY_INSTALL_TYPE`     | [Install type](#install-types) of Liberty. Valid options: `ol` and `none`. Defaults to `ol`.                                                                                                                                                                        |
| `$BP_LIBERTY_VERSION`          | The version of Liberty to install. Defaults to the latest version of the runtime.                                                                                                                                                                                   |
| `$BP_LIBERTY_PROFILE`          | The Liberty profile to use. Defaults to `full`.                                                                                                                                                                                                                     |
| `$BP_LIBERTY_SERVER_NAME`      | Name of the server to use. Defaults to `defaultServer` when building an application. If building a packaged server and there is only one bundled server present, then the buildpack will use that.                                                                  |
| `$BP_LIBERTY_CONTEXT_ROOT`     | If the [server.xml](#bindings) does not have an [application](https://openliberty.io/docs/latest/reference/config/application.html) named `app` defined, then the buildpack will generate one and use this value as the context root. Defaults to the value of `/`. |
| `$BP_LIBERTY_FEATURES`         | Space separated list of Liberty features to be installed with the Liberty runtime.                                                                                                                                                                                  |
| `$BP_LIBERTY_EXT_CONF_SHA256`  | The SHA256 hash of the external configuration package.                                                                                                                                                                                                              |
| `$BP_LIBERTY_EXT_CONF_STRIP`   | The number of directory levels to strip from the external configuration package. Defaults to 0.                                                                                                                                                                     |
| `$BP_LIBERTY_EXT_CONF_URI`     | The download URI of the external configuration package.                                                                                                                                                                                                             |
| `$BP_LIBERTY_EXT_CONF_VERSION` | The version of the external configuration package.                                                                                                                                                                                                                  |
| `$BPL_LIBERTY_LOG_LEVEL`       | Sets the [logging](https://openliberty.io/docs/21.0.0.11/log-trace-configuration.html#configuaration) level. If not set, attempts to get the buildpack's log level. If unable, defaults to `INFO`                                                                   |

### Default Configurations that Vary from Open Liberty's Default

By default, the Liberty buildpack will log in `json` format. This will aid in log ingestion. Due to design decisions from the Open Liberty team, setting this format to any other value will prevent all log types from being sent to `stdout` and will instead go to `messages.log`. In addition, the log sources that will go to stdout are `message,trace,accessLog,ffdc,audit`.

All of these defaults can be overridden by setting the appropriate properties found in Open Liberty's [documentation](https://openliberty.io/docs/21.0.0.11/log-trace-configuration.html). They can be set as environment variables, or in [`bootstrap.properties`](#bindings).

## Including Server Configuration in the Application Image

The following server configuration files can be included in the application image:

* server.xml
* server.env
* bootstrap.properties
* jvm.options

At the moment, these files can only be included in the build by telling the Maven or Gradle buildpacks to provide them.
For example, to provide server configuration in the `src/main/liberty/config`, set one of the following environment
variables in your `pack build` command.

### Including Server Configuration with Maven Applications

```
--env BP_MAVEN_BUILT_ARTIFACT="target/*.[ejw]ar src/main/liberty/config/*"
```

### Including Server Configuration with Gradle Applications

```
--env BP_GRADLE_BUILT_ARTIFACT="build/libs/*.[ejw]ar src/main/liberty/config/*"
```

## Install Types

The different installation types that can be configured are:

* `ol`: This will download an Open Liberty runtime and use it when deploying the container.
* `none`: This will use the Liberty runtime provided in the stack run image. Requires a custom builder.

## Bindings

The buildpack accepts the following bindings:

### Type: `liberty`

| Key                    | Value             | Description                                                                                                                                                                   |
| ---------------------- | ----------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `server.xml`           | `<file-contents>` | This file will replace the `defaultServer`'s `server.xml` and is not subject to any post-processing; therefore, any variable references therein must be resolvable. Optional. |
| `bootstrap.properties` | `<file-contents>` | This file will replace the `defaultServer`'s `bootstrap.properties`. This is one place to define variables used by `server.xml`. Optional.                                    |

### Type: `dependency-mapping`

| Key                   | Value   | Description                                                                                       |
| --------------------- | ------- | ------------------------------------------------------------------------------------------------- |
| `<dependency-digest>` | `<uri>` | If needed, the buildpack will fetch the dependency with digest `<dependency-digest>` from `<uri>` |

## Installing Features

You can install features by setting `$BP_LIBERTY_FEATURES` to be a space separate list of the features you want to install. For example, `BP_LIBERTY_FEATURES='jdbc-4.3 el-3.0'`. You can see a full list of available features in the [Liberty documentation on Features](https://openliberty.io/docs/22.0.0.2/reference/feature/feature-overview.html).

Features are by default downloaded from Maven Central. You can control this behavior using the [standard environment variables for controlling `featureUtility`](https://openliberty.io/docs/22.0.0.2/reference/command/featureUtility-modifications.html). For example, `FEATURE_REPO_URL`, `http_proxy` and `https_proxy`.

### Using Custom Features

Custom features can be configured on the server as well by supplying an external configuration package containing the feature JAR and manifest along with a feature descriptor.

#### Feature Manifest

The feature manifest is a TOML file called `features.toml` containing a list of `features` that should be installed on the server.

A feature has the properties:

* `name`: Name of the feature to enable. Use the symbolic name of the feature that you would use when enabling the feature in the `server.xml`.
* `uri`: URI of where to find the feature. The `file` scheme is the only supported scheme at the moment.
* `version`: Version of the feature.
* `dependencies`: List of features that the custom feature depends on, if any.

#### Example Feature Manifest

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

The external configuration package can be provided to the build by providing the `BP_LIBERTY_EXT_CONF_*`
environment variables to the build. For example, if the external configuration is hosted on a web server, you can use:

```console
pack build --path myapp --env BP_JAVA_APP_SERVER=liberty --env BP_LIBERTY_EXT_CONF_URI=https://example.com/liberty-conf.tar.gz --env BP_LIBERTY_EXT_CONF_VERSION=1.0.0 --env BP_LIBERTY_EXT_CONF_SHA256=953e665e4126b75fecb375c88c51a1ddcf4d12474d43576323862d422e625517 myapp
```

If you'd like to provide the configuration as a file, you can do so by mounting the external configuration in the
container and then providing the path to the external configuration like so:

```console
pack build --path myapp --env BP_JAVA_APP_SERVER=liberty --env BP_LIBERTY_EXT_CONF_URI=file:///path/to/conf/liberty-conf.tar.gz --env BP_LIBERTY_EXT_CONF_VERSION=1.0.0 --env BP_LIBERTY_EXT_CONF_SHA256=953e665e4126b75fecb375c88c51a1ddcf4d12474d43576323862d422e625517 myapp
```

## Building from a Liberty Server

The buildpack can build from Liberty server installation directory or from a packaged server that was created using the
`server package` [command](https://openliberty.io/docs/latest/reference/command/server-package.html).

### Building from a Liberty Server Installation Directory

To build from a Liberty server installation, change your working directory to the installation root containing the `wlp`
directory and run

```console
pack build --env BP_JAVA_APP_SERVER=liberty myapp
```

### Building from a Packaged Server

A packaged server is created using the `server package` command of the Liberty runtime. To create a packaged server,
run this command from your Liberty installation's `wlp` directory:

```console
bin/server package defaultServer --include=usr
```

The packaged server can then be supplied to the build by using the `--path` argument like so:

```console
pack build --env BP_JAVA_APP_SERVER=liberty --path <packaged-server-zip-path> myapp
```

## Installing iFixes

Liberty iFixes can be applied using a volume mount to `/ifixes`. [See the additional docs for details](docs/installing-ifixes.md). 

## License

This buildpack is released under version 2.0 of the [Apache License][a].

[a]: http://www.apache.org/licenses/LICENSE-2.0
