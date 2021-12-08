# `gcr.io/paketo-buildpacks/open-liberty`

The Paketo Open Liberty Buildpack is a Cloud Native Buildpack that contributes Open Liberty for Java EE support.

## Behavior

This buildpack will participate all the following conditions are met

```
(
  <APPLICATION_ROOT>/WEB-INF exists OR
  <APPLICATION_ROOT>/server.xml exists OR
  <APPLICATION_ROOT>/wlp/usr/servers/*/server.xml exists
) AND (
  Main-Class is NOT defined in the mainfest
)
```

The buildpack will do the following:

* Requests that a JRE be installed
* Contribute an Open Liberty runtime to `wlp.install.dir`
* Create a server called `defaultServer`
* Contributes `web` process type
* At launch time, symlinks `<APPLICATION_ROOT>` to `${wlp.install.dir}/usr/servers/defaultServer/dropins/<APPLICATION_ROOT_BASENAME>`.

The buildpack will support all available profiles of the two most recent versions of the Open Liberty runtime. Note that because the Open Liberty versioning scheme is not conformant semantic versioning, an Open Liberty version like `21.0.0.11` is defined here as `21.0.11`, and should be referenced as such.

## Configuration

| Environment Variable          | Description                                                                                                                                                                                       |
| ----------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `$BP_OPENLIBERTY_VERSION`     | The version of Open Liberty to install. Defaults to the latest version of the runtime.                                                                                                            |
| `$BP_OPENLIBERTY_PROFILE`     | The Open Liberty profile to use. Defaults to `full`.                                                                                                                                              |
| `$BP_OPENLIBERTY_WEBINF_PATH` | If the package's WEB-INF directory is not located at the root of the package, specify it here.                                                                                                    |
| `$BPL_OPENLIBERTY_APP_NAME`   | If the [server.xml](#bindings) does not specify a context root, Open Liberty will use this value as the context root. Defaults to the value of `$CNB_APP_DIR`, currently `workspace`.             |
| `$BPL_OPENLIBERTY_LOG_LEVEL`  | Sets the [logging](https://openliberty.io/docs/21.0.0.11/log-trace-configuration.html#configuaration) level. If not set, attempts to get the buildpack's log level. If unable, defaults to `INFO` |

### Default Configurations that Vary from Open Liberty's Default

By default, the Open Liberty buildpack will log in `json` format. This will aid in log ingestion. Due to design decisions from the Open Liberty team, setting this format to any other value will prevent all log types from being sent to `stdout` and will instead go to `messages.log`. In addition, the log sources that will go to stdout are `message,trace,accessLog,ffdc,audit`.

All of these defaults can be overriden by setting the appropriate properties found in Open Liberty's [documentation](https://openliberty.io/docs/21.0.0.11/log-trace-configuration.html). They can be set as environment variables, or in [`bootstrap.properties`](#bindings).

## Bindings

The Open Liberty buildpack will accept a single [binding](https://paketo.io/docs/howto/configuration/#bindings) of type `open-liberty`. There are currently two files supported in the binding:
1. `server.xml`. If specified, this file will replace the `defaultServer`'s `server.xml` and is not subject to any post-processing; therefore, any variable references therein must be resolvable.
1. `bootstrap.properties`. If specified, this file will replace the `defaultServer`'s `bootstrap.properties`. This is one place to define variables used by `server.xml`.

Both files are optional, as is the binding itself.

## License

This buildpack is released under version 2.0 of the [Apache License][a].

[a]: http://www.apache.org/licenses/LICENSE-2.0

