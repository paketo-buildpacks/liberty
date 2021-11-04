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
| Environment Variable | Description
| -------------------- | -----------
| `$BP_OPENLIBERTY_VERSION` | The version of Open Liberty to install. Defaults to the latest version of the runtime. 
| `$BP_OPENLIBERTY_PROFILE` | The Open Liberty profile to use. Defaults to `full`.

## License
This buildpack is released under version 2.0 of the [Apache License][a].

[a]: http://www.apache.org/licenses/LICENSE-2.0

