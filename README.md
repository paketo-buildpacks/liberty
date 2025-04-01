# `paketobuildpacks/liberty`

The Paketo Buildpack for Liberty is a Cloud-Native Buildpack that contributes Open Liberty or WebSphere Liberty for Java EE support.

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
* Create a server.xml with the default features for the profile selected
* If a web application was built, it will symlink `<APPLICATION_ROOT>` to `<WLP_USR_DIR>/servers/<SERVER_NAME>/apps/app`
* If a Liberty server was built, it will symlink `<APPLICATION_ROOT>` to `<WLP_USR_DIR>`

The buildpack will support all available profiles of the most recent versions of the Liberty runtime. Because the Liberty versioning scheme is not conformant to semantic versioning, a Liberty version like `22.0.0.2` is defined here as `22.0.2`, and should be referenced as such.

## Configuration

| Environment Variable                  | Description                                                                                                                                                                                                                                                                                                                                            |
|---------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `$BP_JAVA_APP_SERVER`                 | The application server to use. It defaults to `` (empty string) which means that order dictates which Java application server is installed. The first Java application server buildpack to run will be picked.                                                                                                                                         |
| `$BP_LIBERTY_INSTALL_TYPE`            | [Install type](#install-types) of Liberty. Valid options: `ol`, `wlp`, and `none`. Defaults to `ol`.                                                                                                                                                                                                                                                   |
| `$BP_LIBERTY_VERSION`                 | The version of Liberty to install. Defaults to the latest version of the runtime. To see what version is available with your version of the buildpack, please see the [release notes][release-notes]. At present, only the latest version is supported, and you need to use an older version of the buildpack if you want an older version of Liberty. |
| `$BP_LIBERTY_PROFILE`                 | The Liberty profile to use. Defaults to `kernel`.                                                                                                                                                                                                                                                                                                      |
| `$BP_LIBERTY_SERVER_NAME`             | Name of the server to use. Defaults to `defaultServer` when building an application. If building a packaged server and there is only one bundled server present, then the buildpack will use that.                                                                                                                                                     |
| `$BP_LIBERTY_CONTEXT_ROOT`            | The context root to use for the application. Defaults to the context root for the [application][app-config] if defined in the [server.xml](#bindings). Otherwise, it defaults to `/`.                                                                                                                                                                  |
| `$BP_LIBERTY_FEATURES`                | Space separated list of Liberty features to be installed with the Liberty runtime. Supports any valid Liberty feature. See the [Liberty Documentation][liberty-doc] for available features.                                                                                                                                                            |
| `BP_LIBERTY_FEATURE_INSTALL_DISABLED` | Disable running the feature installer. Defaults to `false`.                                                                                                                                                                                                                                                                                            |
| `$BPL_LIBERTY_LOG_LEVEL`              | Sets the [logging](https://openliberty.io/docs/21.0.0.11/log-trace-configuration.html#configuaration) level. If not set, attempts to get the buildpack's log level. If unable, defaults to `INFO`                                                                                                                                                      |

[release-notes]: https://github.com/paketo-buildpacks/liberty/releases
[app-config]: https://openliberty.io/docs/latest/reference/config/application.html
[liberty-doc]: https://openliberty.io/docs/latest/reference/feature/feature-overview.html

### Profiles

Valid profiles for Open Liberty are:

* full
* kernel
* jakartaee10
* javaee8
* webProfile9
* webProfile8
* microProfile7
* microProfile4

Valid profiles for WebSphere Liberty are:

* kernel
* jakartaee10
* javaee8
* javaee7
* webProfile10
* webProfile8
* webProfile7

### Default Configurations that Vary from Liberty's Default

By default, the Liberty buildpack will log in `json` format. This will aid in log ingestion. Due to design decisions from the Liberty team, setting this format to any other value will prevent all log types from being sent to `stdout` and will instead go to `messages.log`. In addition, the log sources that will go to stdout are `message,trace,accessLog,ffdc,audit`.

All of these defaults can be overridden by setting the appropriate properties found in Liberty's [documentation](https://openliberty.io/docs/21.0.0.11/log-trace-configuration.html). They can be set as environment variables, or in [`bootstrap.properties`](#bindings).

## Including Server Configuration in the Application Image

The following server configuration files can be included in the application image:

* server.xml
* server.env
* bootstrap.properties

**IMPORTANT NOTE:** Do not put secrets in any of these configuration files! The files will be included in the resulting
image and can leak your secrets. See [Configuring Secrets](#configuring-secrets) for information on how to provide
secrets in your configuration.

At the moment, these files can only be included in the build by telling the Maven or Gradle buildpacks to provide them. Thus this method of including server configuration can only be performed when building from source code, it will not work when building with a pre-compiled WAR file.

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

### Providing Application Config in server.xml

Any application configuration provided in the `server.xml` must have an `id` set. This is required for the Liberty buildpack to provide additional configuration (e.g., updating the application's location).

For example:

```xml
<application id="myapp" name="myapp" context-root="/my-app">
  <classloader commonLibraryRef="my-lib-ref"/>
</application>
```

## Configuring Secrets

Sensitive data should not be included in any of the configuration files provided during the build. The files will be
included in the application image which can leak your secrets.

Instead, set the secrets in your `bootstrap.properties` file and provide it to the application container as a binding.

For example, to configure a custom password for Liberty's default keystore, you can add the following line to your
`server.xml`:

```xml
<keyStore id="defaultKeyStore" password="${keystore.password}" />
```

The property `keystore.password` can then be configured in the application image via a binding of type `liberty` under
the `bootstrap.properties` key.

## Install Types

The different installation types that can be configured are:

* `ol`: This will download an Open Liberty runtime and use it when deploying the container.
* `wlp`: This will download a WebSphere Liberty runtime and use it when deploying the container.
* `none`: This will use the Liberty runtime provided in the stack run image. Requires a custom builder.

## Bindings

The buildpack accepts the following bindings:

### Type: `liberty`

| Key                    | Value             | Description                                                                                                                                                                   |
|------------------------|-------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `server.xml`           | `<file-contents>` | This file will replace the `defaultServer`'s `server.xml` and is not subject to any post-processing; therefore, any variable references therein must be resolvable. Optional. |
| `bootstrap.properties` | `<file-contents>` | This file will replace the `defaultServer`'s `bootstrap.properties`. This is one place to define variables used by `server.xml`. Optional.                                    |

### Type: `dependency-mapping`

| Key                   | Value   | Description                                                                                       |
|-----------------------|---------|---------------------------------------------------------------------------------------------------|
| `<dependency-digest>` | `<uri>` | If needed, the buildpack will fetch the dependency with digest `<dependency-digest>` from `<uri>` |

## Installing Features

You can install features by setting `$BP_LIBERTY_FEATURES` to be a space separate list of the features you want to install. For example, `BP_LIBERTY_FEATURES='jdbc-4.3 el-3.0'`. You can see a full list of available features in the [Liberty documentation on Features](https://openliberty.io/docs/22.0.0.2/reference/feature/feature-overview.html).

Features are by default downloaded from Maven Central. You can control this behavior using the [standard environment variables for controlling `featureUtility`](https://openliberty.io/docs/22.0.0.2/reference/command/featureUtility-modifications.html). For example, `FEATURE_REPO_URL`, `http_proxy` and `https_proxy`.

### Using Custom Features

Custom features can be configured on the server as well using a volume mount to `/features` that contains the feature JARs and manifests along with a feature descriptor.

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
  uri = "file:///features/cache.dummy_1.0.0.jar"
  version = "1.0.0"
  dependencies = ["distributedMap-1.0"]
```

Using this feature description, the Liberty buildpack will look for the feature JAR in the volume mounted on
`/features` at the path `features/cache.dummy_1.0.0.jar`. The buildpack also assumes that the feature manifest file will
be at the path `features/cache.dummy_1.0.0.mf`.

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

The custom features can then be provided to the build by mounting the feature directory to `/features`:

```console
pack build --path myapp --env BP_JAVA_APP_SERVER=liberty --volume /Users/hwibell/Development/paketo-buildpacks/liberty-e2e.bak/data/conf/features:/features myapp
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
