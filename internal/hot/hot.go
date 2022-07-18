package hot

import "github.com/evanw/esbuild/internal/logger"

func devClient() string {
	return `var protocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
	var hostname = window.location.hostname;
	var port = window.location.port;
	var pathname = "/espack-socket";
	var connectionUrl = protocol + '://' + hostname + ':' + port + pathname
	var connection = new WebSocket(connectionUrl);
	var heartbeatTimer = -1;
	connection.onclose = function () {
		clearTimeout(heartbeatTimer)
		if (typeof console !== 'undefined' && typeof console.info === 'function') {
			console.info('The development server has disconnected.\nRefresh the page if necessary.');
		}
	}; // Remember some state related to hot module replacement.


	var isFirstCompilation = false;
	var mostRecentCompilationHash = null;
	var hasCompileErrors = false;

	var hadRuntimeError = false;

	function setupHeartbeat() {
		function ping() {
			heartbeatTimer = setTimeout(()=> {
				connection.send("ping")
				ping();
			}, 3000)
		}
		connection.onopen = ()=> {
			ping()
		}
	}

	setupHeartbeat()

	function clearOutdatedErrors() {
		// Clean up outdated compile errors, if any.
		if (typeof console !== 'undefined' && typeof console.clear === 'function') {
			if (hasCompileErrors) {
				console.clear();
			}
		}
	} // Successful compilation.


	function handleSuccess() {
		clearOutdatedErrors();
		var isHotUpdate = !isFirstCompilation;
		isFirstCompilation = false;
		hasCompileErrors = false; // Attempt to apply hot updates or reload.

		if (isHotUpdate) {
			tryApplyUpdates(function onHotUpdateSuccess() {
				console.info('success update!')
			});
		}
	} // Compilation with warnings (e.g. ESLint).


	function handleWarnings(warnings) {
		clearOutdatedErrors();
		var isHotUpdate = !isFirstCompilation;
		isFirstCompilation = false;
		hasCompileErrors = false;

		console.warn(warnings)

		if (isHotUpdate) {
			tryApplyUpdates(function onSuccessfulHotUpdate() {
				console.info('success update!')
			});
		}
	} // Compilation with errors (e.g. syntax error or missing modules).


	function handleErrors(errors) {
		clearOutdatedErrors();
		isFirstCompilation = false;
		hasCompileErrors = true; // "Massage" webpack messages.

		console.error(errors)

	}



	function handleAvailableHash(hash) {
		// Update last known compilation hash.
		mostRecentCompilationHash = hash;
	} // Handle messages from the server.


	connection.onmessage = function (e) {
		var message = JSON.parse(e.data);

		switch (message.type) {
			case 'hash':
				handleAvailableHash(message.data);
				break;

			case 'still-ok':
			case 'ok':
				handleSuccess();
				break;

			case 'content-changed':
				window.location.reload();
				break;

			case 'warnings':
				handleWarnings(message.data);
				break;

			case 'errors':
				handleErrors(message.data);
				break;

			default: // Do nothing.

		}
	}; // Is there a newer version of this code available?


	function isUpdateAvailable() {
		/* globals __webpack_hash__ */
		// __webpack_hash__ is the hash of the current compilation.
		// It's a global variable injected by webpack.
		return mostRecentCompilationHash !== require.h();
	} // webpack disallows updates in other states.


	function canApplyUpdates() {
		return module.hot.status() === 'idle';
	} // Attempt to update code on the fly, fall back to a hard reload.


	function tryApplyUpdates(onHotUpdateSuccess) {
		if (false) {}

		if (!isUpdateAvailable() || !canApplyUpdates()) {
			return;
		}

		function handleApplyUpdates(err, updatedModules) {
			// NOTE: This var is injected by Webpack's DefinePlugin, and is a boolean instead of string.
			const hasReactRefresh = false;
			const wantsForcedReload = err || !updatedModules || hadRuntimeError; // React refresh can handle hot-reloading over errors.
			if(err) {
				console.error(err)
			}
			if (!hasReactRefresh && wantsForcedReload) {
				window.location.reload();
				return;
			}

			if (typeof onHotUpdateSuccess === 'function') {
				// Maybe we want to do something.
				onHotUpdateSuccess();
			}

			if (isUpdateAvailable()) {
				// While we were updating, there was a new update! Do it again.
				tryApplyUpdates();
			}
		}


		var result = module.hot.check(true, handleApplyUpdates); // // webpack 2 returns a Promise instead of invoking a callback

		if (result && result.then) {
			result.then(function (updatedModules) {
				handleApplyUpdates(null, updatedModules);
			}, function (err) {
				handleApplyUpdates(err, null);
			});
		}
	}`
}

func GenerateDevClient() logger.Source {
	return logger.Source{
		Index:          ^uint32(0) - 1,
		KeyPath:        logger.Path{Text: "<dev_client>"},
		PrettyPath:     "<dev_client>",
		IdentifierName: "dev_client",
		Contents:       devClient(),
	}
}
