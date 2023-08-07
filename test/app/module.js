webpackJsonp.push([["0"], {"<dev_client>": (function(module, exports, require) {
  var protocol = window.location.protocol === "https:" ? "wss" : "ws", hostname = window.location.hostname, port = window.location.port, pathname = "/espack-socket", connectionUrl = protocol + "://" + hostname + ":" + port + pathname, connection = new WebSocket(connectionUrl), heartbeatTimer = -1;
  connection.onclose = function() {
    clearTimeout(heartbeatTimer), typeof console < "u" && typeof console.info == "function" && console.info("The development server has disconnected.\nRefresh the page if necessary.");
  };
  var isFirstCompilation = false, mostRecentCompilationHash = null, hasCompileErrors = false, hadRuntimeError = false;
  function setupHeartbeat() {
    function ping() {
      heartbeatTimer = setTimeout(() => {
        connection.send("ping"), ping();
      }, 3e3);
    }
    connection.onopen = () => {
      ping();
    };
  }
  setupHeartbeat();
  function clearOutdatedErrors() {
    typeof console < "u" && typeof console.clear == "function" && hasCompileErrors && console.clear();
  }
  function handleSuccess() {
    clearOutdatedErrors();
    var isHotUpdate = !isFirstCompilation;
    isFirstCompilation = false, hasCompileErrors = false, isHotUpdate && tryApplyUpdates(function() {
      console.info("success update!");
    });
  }
  function handleWarnings(warnings) {
    clearOutdatedErrors();
    var isHotUpdate = !isFirstCompilation;
    isFirstCompilation = false, hasCompileErrors = false, console.warn(warnings), isHotUpdate && tryApplyUpdates(function() {
      console.info("success update!");
    });
  }
  function handleErrors(errors) {
    clearOutdatedErrors(), isFirstCompilation = false, hasCompileErrors = true, console.error(errors);
  }
  function handleAvailableHash(hash) {
    mostRecentCompilationHash = hash;
  }
  connection.onmessage = function(e) {
    var message = JSON.parse(e.data);
    switch (message.type) {
      case "hash":
        handleAvailableHash(message.data);
        break;
      case "still-ok":
      case "ok":
        handleSuccess();
        break;
      case "content-changed":
        window.location.reload();
        break;
      case "warnings":
        handleWarnings(message.data);
        break;
      case "errors":
        handleErrors(message.data);
        break;
      default:
    }
  };
  function isUpdateAvailable() {
    return mostRecentCompilationHash !== require.h();
  }
  function canApplyUpdates() {
    return module.hot.status() === "idle";
  }
  function tryApplyUpdates(onHotUpdateSuccess) {
    if (!isUpdateAvailable() || !canApplyUpdates())
      return;
    function handleApplyUpdates(err, updatedModules) {
      const wantsForcedReload = err || !updatedModules || hadRuntimeError;
      if (err && console.error(err), wantsForcedReload) {
        window.location.reload();
        return;
      }
      typeof onHotUpdateSuccess == "function" && onHotUpdateSuccess(), isUpdateAvailable() && tryApplyUpdates();
    }
    var result = module.hot.check(true, handleApplyUpdates);
    result && result.then && result.then(function(updatedModules) {
      handleApplyUpdates(null, updatedModules);
    }, function(err) {
      handleApplyUpdates(err, null);
    });
  }
})}])