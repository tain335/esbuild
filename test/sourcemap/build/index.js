(function(modules) {
  function webpackJsonpCallback(data) {
    var chunkIds = data[0];
    var moreModules = data[1];
    var executeModules = data[2];
    var moduleId, chunkId, i = 0, resolves = [];
    for (; i < chunkIds.length; i++) {
      chunkId = chunkIds[i];
      if (Object.prototype.hasOwnProperty.call(installedChunks, chunkId) && installedChunks[chunkId]) {
        resolves.push(installedChunks[chunkId][0]);
      }
      installedChunks[chunkId] = 0;
    }
    for (moduleId in moreModules) {
      if (Object.prototype.hasOwnProperty.call(moreModules, moduleId)) {
        modules[moduleId] = moreModules[moduleId];
      }
    }
    if (parentJsonpFunction)
      parentJsonpFunction(data);
    while (resolves.length) {
      resolves.shift()();
    }
    deferredModules.push.apply(deferredModules, executeModules || []);
    return checkDeferredModules();
  }
  ;
  function checkDeferredModules() {
    var result;
    for (var i = 0; i < deferredModules.length; i++) {
      var deferredModule = deferredModules[i];
      var fulfilled = true;
      for (var j = 1; j < deferredModule.length; j++) {
        var depId = deferredModule[j];
        if (installedChunks[depId] !== 0)
          fulfilled = false;
      }
      if (fulfilled) {
        deferredModules.splice(i--, 1);
        result = __webpack_require__(__webpack_require__.s = deferredModule[0]);
      }
    }
    return result;
  }
  function hotDisposeChunk(chunkId) {
    delete installedChunks[chunkId];
  }
  var parentHotUpdateCallback = this["webpackHotUpdate"];
  this["webpackHotUpdate"] = // eslint-disable-next-line no-unused-vars
  function webpackHotUpdateCallback(chunkId, moreModules) {
    hotAddUpdateChunk(chunkId, moreModules);
    if (parentHotUpdateCallback)
      parentHotUpdateCallback(chunkId, moreModules);
  };
  function hotDownloadUpdateChunk(chunkId) {
    var script = document.createElement("script");
    script.charset = "utf-8";
    script.src = __webpack_require__.p + "" + chunkId + "." + hotCurrentHash + ".hot-update.js";
    if (null)
      script.crossOrigin = null;
    document.head.appendChild(script);
  }
  function hotDownloadManifest(requestTimeout) {
    requestTimeout = requestTimeout || 1e4;
    return new Promise(function(resolve, reject) {
      if (typeof XMLHttpRequest === "undefined") {
        return reject(new Error("No browser support"));
      }
      try {
        var request = new XMLHttpRequest();
        var requestPath = __webpack_require__.p + "" + hotCurrentHash + ".hot-update.json";
        request.open("GET", requestPath, true);
        request.timeout = requestTimeout;
        request.send(null);
      } catch (err) {
        return reject(err);
      }
      request.onreadystatechange = function() {
        if (request.readyState !== 4)
          return;
        if (request.status === 0) {
          reject(
            new Error("Manifest request to " + requestPath + " timed out.")
          );
        } else if (request.status === 404) {
          resolve();
        } else if (request.status !== 200 && request.status !== 304) {
          reject(new Error("Manifest request to " + requestPath + " failed."));
        } else {
          try {
            var update = JSON.parse(request.responseText);
          } catch (e) {
            reject(e);
            return;
          }
          resolve(update);
        }
      };
    });
  }
  var hotApplyOnUpdate = true;
  var hotCurrentHash = "3my0gcm1ml1adp2f";
  var hotRequestTimeout = 1e4;
  var hotCurrentModuleData = {};
  var hotCurrentChildModule;
  var hotCurrentParents = [];
  var hotCurrentParentsTemp = [];
  function hotCreateRequire(moduleId) {
    var me = installedModules[moduleId];
    if (!me)
      return __webpack_require__;
    var fn = function(request) {
      if (me.hot.active) {
        if (installedModules[request]) {
          if (installedModules[request].parents.indexOf(moduleId) === -1) {
            installedModules[request].parents.push(moduleId);
          }
        } else {
          hotCurrentParents = [moduleId];
          hotCurrentChildModule = request;
        }
        if (me.children.indexOf(request) === -1) {
          me.children.push(request);
        }
      } else {
        console.warn(
          "[HMR] unexpected require(" + request + ") from disposed module " + moduleId
        );
        hotCurrentParents = [];
      }
      return __webpack_require__(request);
    };
    var ObjectFactory = function ObjectFactory(name) {
      return {
        configurable: true,
        enumerable: true,
        get: function() {
          return __webpack_require__[name];
        },
        set: function(value) {
          __webpack_require__[name] = value;
        }
      };
    };
    for (var name in __webpack_require__) {
      if (Object.prototype.hasOwnProperty.call(__webpack_require__, name) && name !== "e" && name !== "t") {
        Object.defineProperty(fn, name, ObjectFactory(name));
      }
    }
    fn.e = function(chunkId) {
      if (hotStatus === "ready")
        hotSetStatus("prepare");
      hotChunksLoading++;
      return __webpack_require__.e(chunkId).then(finishChunkLoading, function(err) {
        finishChunkLoading();
        throw err;
      });
      function finishChunkLoading() {
        hotChunksLoading--;
        if (hotStatus === "prepare") {
          if (!hotWaitingFilesMap[chunkId]) {
            hotEnsureUpdateChunk(chunkId);
          }
          if (hotChunksLoading === 0 && hotWaitingFiles === 0) {
            hotUpdateDownloaded();
          }
        }
      }
    };
    fn.t = function(value, mode) {
      if (mode & 1)
        value = fn(value);
      return __webpack_require__.t(value, mode & ~1);
    };
    return fn;
  }
  function hotCreateModule(moduleId) {
    var hot = {
      // private stuff
      _acceptedDependencies: {},
      _declinedDependencies: {},
      _selfAccepted: false,
      _selfDeclined: false,
      _selfInvalidated: false,
      _disposeHandlers: [],
      _main: hotCurrentChildModule !== moduleId,
      // Module API
      active: true,
      accept: function(dep, callback) {
        if (dep === void 0)
          hot._selfAccepted = true;
        else if (typeof dep === "function")
          hot._selfAccepted = dep;
        else if (typeof dep === "object")
          for (var i = 0; i < dep.length; i++)
            hot._acceptedDependencies[dep[i]] = callback || function() {
            };
        else
          hot._acceptedDependencies[dep] = callback || function() {
          };
      },
      decline: function(dep) {
        if (dep === void 0)
          hot._selfDeclined = true;
        else if (typeof dep === "object")
          for (var i = 0; i < dep.length; i++)
            hot._declinedDependencies[dep[i]] = true;
        else
          hot._declinedDependencies[dep] = true;
      },
      dispose: function(callback) {
        hot._disposeHandlers.push(callback);
      },
      addDisposeHandler: function(callback) {
        hot._disposeHandlers.push(callback);
      },
      removeDisposeHandler: function(callback) {
        var idx = hot._disposeHandlers.indexOf(callback);
        if (idx >= 0)
          hot._disposeHandlers.splice(idx, 1);
      },
      invalidate: function() {
        this._selfInvalidated = true;
        switch (hotStatus) {
          case "idle":
            hotUpdate = {};
            hotUpdate[moduleId] = modules[moduleId];
            hotSetStatus("ready");
            break;
          case "ready":
            hotApplyInvalidatedModule(moduleId);
            break;
          case "prepare":
          case "check":
          case "dispose":
          case "apply":
            (hotQueuedInvalidatedModules = hotQueuedInvalidatedModules || []).push(moduleId);
            break;
          default:
            break;
        }
      },
      // Management API
      check: hotCheck,
      apply: hotApply,
      status: function(l) {
        if (!l)
          return hotStatus;
        hotStatusHandlers.push(l);
      },
      addStatusHandler: function(l) {
        hotStatusHandlers.push(l);
      },
      removeStatusHandler: function(l) {
        var idx = hotStatusHandlers.indexOf(l);
        if (idx >= 0)
          hotStatusHandlers.splice(idx, 1);
      },
      //inherit from previous dispose call
      data: hotCurrentModuleData[moduleId]
    };
    hotCurrentChildModule = void 0;
    return hot;
  }
  var hotStatusHandlers = [];
  var hotStatus = "idle";
  function hotSetStatus(newStatus) {
    hotStatus = newStatus;
    for (var i = 0; i < hotStatusHandlers.length; i++)
      hotStatusHandlers[i].call(null, newStatus);
  }
  var hotWaitingFiles = 0;
  var hotChunksLoading = 0;
  var hotWaitingFilesMap = {};
  var hotRequestedFilesMap = {};
  var hotAvailableFilesMap = {};
  var hotDeferred;
  var hotUpdate, hotUpdateNewHash, hotQueuedInvalidatedModules;
  function toModuleId(id) {
    var isNumber = +id + "" === id;
    return isNumber ? +id : id;
  }
  function hotCheck(apply) {
    if (hotStatus !== "idle") {
      throw new Error("check() is only allowed in idle status");
    }
    hotApplyOnUpdate = apply;
    hotSetStatus("check");
    return hotDownloadManifest(hotRequestTimeout).then(function(update) {
      if (!update) {
        hotSetStatus(hotApplyInvalidatedModules() ? "ready" : "idle");
        return null;
      }
      hotRequestedFilesMap = {};
      hotWaitingFilesMap = {};
      hotAvailableFilesMap = update.c;
      hotUpdateNewHash = update.h;
      hotSetStatus("prepare");
      var promise = new Promise(function(resolve, reject) {
        hotDeferred = {
          resolve,
          reject
        };
      });
      hotUpdate = {};
      for (var chunkId in installedChunks) {
        hotEnsureUpdateChunk(chunkId);
      }
      if (hotStatus === "prepare" && hotChunksLoading === 0 && hotWaitingFiles === 0) {
        hotUpdateDownloaded();
      }
      return promise;
    });
  }
  function hotAddUpdateChunk(chunkId, moreModules) {
    if (!hotAvailableFilesMap[chunkId] || !hotRequestedFilesMap[chunkId])
      return;
    hotRequestedFilesMap[chunkId] = false;
    for (var moduleId in moreModules) {
      if (Object.prototype.hasOwnProperty.call(moreModules, moduleId)) {
        hotUpdate[moduleId] = moreModules[moduleId];
      }
    }
    if (--hotWaitingFiles === 0 && hotChunksLoading === 0) {
      hotUpdateDownloaded();
    }
  }
  function hotEnsureUpdateChunk(chunkId) {
    if (!hotAvailableFilesMap[chunkId]) {
      hotWaitingFilesMap[chunkId] = true;
    } else {
      hotRequestedFilesMap[chunkId] = true;
      hotWaitingFiles++;
      hotDownloadUpdateChunk(chunkId);
    }
  }
  function hotUpdateDownloaded() {
    hotSetStatus("ready");
    var deferred = hotDeferred;
    hotDeferred = null;
    if (!deferred)
      return;
    if (hotApplyOnUpdate) {
      Promise.resolve().then(function() {
        return hotApply(hotApplyOnUpdate);
      }).then(
        function(result) {
          deferred.resolve(result);
        },
        function(err) {
          deferred.reject(err);
        }
      );
    } else {
      var outdatedModules = [];
      for (var id in hotUpdate) {
        if (Object.prototype.hasOwnProperty.call(hotUpdate, id)) {
          outdatedModules.push(toModuleId(id));
        }
      }
      deferred.resolve(outdatedModules);
    }
  }
  function hotApply(options) {
    if (hotStatus !== "ready")
      throw new Error("apply() is only allowed in ready status");
    options = options || {};
    return hotApplyInternal(options);
  }
  function hotApplyInternal(options) {
    hotApplyInvalidatedModules();
    var cb;
    var i;
    var j;
    var module;
    var moduleId;
    function getAffectedStuff(updateModuleId) {
      var outdatedModules = [updateModuleId];
      var outdatedDependencies = {};
      var queue = outdatedModules.map(function(id) {
        return {
          chain: [id],
          id
        };
      });
      while (queue.length > 0) {
        var queueItem = queue.pop();
        var moduleId = queueItem.id;
        var chain = queueItem.chain;
        module = installedModules[moduleId];
        if (!module || module.hot._selfAccepted && !module.hot._selfInvalidated)
          continue;
        if (module.hot._selfDeclined) {
          return {
            type: "self-declined",
            chain,
            moduleId
          };
        }
        if (module.hot._main) {
          return {
            type: "unaccepted",
            chain,
            moduleId
          };
        }
        for (var i = 0; i < module.parents.length; i++) {
          var parentId = module.parents[i];
          var parent = installedModules[parentId];
          if (!parent)
            continue;
          if (parent.hot._declinedDependencies[moduleId]) {
            return {
              type: "declined",
              chain: chain.concat([parentId]),
              moduleId,
              parentId
            };
          }
          if (outdatedModules.indexOf(parentId) !== -1)
            continue;
          if (parent.hot._acceptedDependencies[moduleId]) {
            if (!outdatedDependencies[parentId])
              outdatedDependencies[parentId] = [];
            addAllToSet(outdatedDependencies[parentId], [moduleId]);
            continue;
          }
          delete outdatedDependencies[parentId];
          outdatedModules.push(parentId);
          queue.push({
            chain: chain.concat([parentId]),
            id: parentId
          });
        }
      }
      return {
        type: "accepted",
        moduleId: updateModuleId,
        outdatedModules,
        outdatedDependencies
      };
    }
    function addAllToSet(a, b) {
      for (var i = 0; i < b.length; i++) {
        var item = b[i];
        if (a.indexOf(item) === -1)
          a.push(item);
      }
    }
    var outdatedDependencies = {};
    var outdatedModules = [];
    var appliedUpdate = {};
    var warnUnexpectedRequire = function warnUnexpectedRequire() {
      console.warn(
        "[HMR] unexpected require(" + result.moduleId + ") to disposed module"
      );
    };
    for (var id in hotUpdate) {
      if (Object.prototype.hasOwnProperty.call(hotUpdate, id)) {
        moduleId = toModuleId(id);
        var result;
        if (hotUpdate[id]) {
          result = getAffectedStuff(moduleId);
        } else {
          result = {
            type: "disposed",
            moduleId: id
          };
        }
        var abortError = false;
        var doApply = false;
        var doDispose = false;
        var chainInfo = "";
        if (result.chain) {
          chainInfo = "\nUpdate propagation: " + result.chain.join(" -> ");
        }
        switch (result.type) {
          case "self-declined":
            if (options.onDeclined)
              options.onDeclined(result);
            if (!options.ignoreDeclined)
              abortError = new Error(
                "Aborted because of self decline: " + result.moduleId + chainInfo
              );
            break;
          case "declined":
            if (options.onDeclined)
              options.onDeclined(result);
            if (!options.ignoreDeclined)
              abortError = new Error(
                "Aborted because of declined dependency: " + result.moduleId + " in " + result.parentId + chainInfo
              );
            break;
          case "unaccepted":
            if (options.onUnaccepted)
              options.onUnaccepted(result);
            if (!options.ignoreUnaccepted)
              abortError = new Error(
                "Aborted because " + moduleId + " is not accepted" + chainInfo
              );
            break;
          case "accepted":
            if (options.onAccepted)
              options.onAccepted(result);
            doApply = true;
            break;
          case "disposed":
            if (options.onDisposed)
              options.onDisposed(result);
            doDispose = true;
            break;
          default:
            throw new Error("Unexception type " + result.type);
        }
        if (abortError) {
          hotSetStatus("abort");
          return Promise.reject(abortError);
        }
        if (doApply) {
          appliedUpdate[moduleId] = hotUpdate[moduleId];
          addAllToSet(outdatedModules, result.outdatedModules);
          for (moduleId in result.outdatedDependencies) {
            if (Object.prototype.hasOwnProperty.call(
              result.outdatedDependencies,
              moduleId
            )) {
              if (!outdatedDependencies[moduleId])
                outdatedDependencies[moduleId] = [];
              addAllToSet(
                outdatedDependencies[moduleId],
                result.outdatedDependencies[moduleId]
              );
            }
          }
        }
        if (doDispose) {
          addAllToSet(outdatedModules, [result.moduleId]);
          appliedUpdate[moduleId] = warnUnexpectedRequire;
        }
      }
    }
    var outdatedSelfAcceptedModules = [];
    for (i = 0; i < outdatedModules.length; i++) {
      moduleId = outdatedModules[i];
      if (installedModules[moduleId] && installedModules[moduleId].hot._selfAccepted && // removed self-accepted modules should not be required
      appliedUpdate[moduleId] !== warnUnexpectedRequire && // when called invalidate self-accepting is not possible
      !installedModules[moduleId].hot._selfInvalidated) {
        outdatedSelfAcceptedModules.push({
          module: moduleId,
          parents: installedModules[moduleId].parents.slice(),
          errorHandler: installedModules[moduleId].hot._selfAccepted
        });
      }
    }
    hotSetStatus("dispose");
    Object.keys(hotAvailableFilesMap).forEach(function(chunkId) {
      if (hotAvailableFilesMap[chunkId] === false) {
        hotDisposeChunk(chunkId);
      }
    });
    var idx;
    var queue = outdatedModules.slice();
    while (queue.length > 0) {
      moduleId = queue.pop();
      module = installedModules[moduleId];
      if (!module)
        continue;
      var data = {};
      var disposeHandlers = module.hot._disposeHandlers;
      for (j = 0; j < disposeHandlers.length; j++) {
        cb = disposeHandlers[j];
        cb(data);
      }
      hotCurrentModuleData[moduleId] = data;
      module.hot.active = false;
      delete installedModules[moduleId];
      delete outdatedDependencies[moduleId];
      for (j = 0; j < module.children.length; j++) {
        var child = installedModules[module.children[j]];
        if (!child)
          continue;
        idx = child.parents.indexOf(moduleId);
        if (idx >= 0) {
          child.parents.splice(idx, 1);
        }
      }
    }
    var dependency;
    var moduleOutdatedDependencies;
    for (moduleId in outdatedDependencies) {
      if (Object.prototype.hasOwnProperty.call(outdatedDependencies, moduleId)) {
        module = installedModules[moduleId];
        if (module) {
          moduleOutdatedDependencies = outdatedDependencies[moduleId];
          for (j = 0; j < moduleOutdatedDependencies.length; j++) {
            dependency = moduleOutdatedDependencies[j];
            idx = module.children.indexOf(dependency);
            if (idx >= 0)
              module.children.splice(idx, 1);
          }
        }
      }
    }
    hotSetStatus("apply");
    if (hotUpdateNewHash !== void 0) {
      hotCurrentHash = hotUpdateNewHash;
      hotUpdateNewHash = void 0;
    }
    hotUpdate = void 0;
    for (moduleId in appliedUpdate) {
      if (Object.prototype.hasOwnProperty.call(appliedUpdate, moduleId)) {
        modules[moduleId] = appliedUpdate[moduleId];
      }
    }
    var error = null;
    for (moduleId in outdatedDependencies) {
      if (Object.prototype.hasOwnProperty.call(outdatedDependencies, moduleId)) {
        module = installedModules[moduleId];
        if (module) {
          moduleOutdatedDependencies = outdatedDependencies[moduleId];
          var callbacks = [];
          for (i = 0; i < moduleOutdatedDependencies.length; i++) {
            dependency = moduleOutdatedDependencies[i];
            cb = module.hot._acceptedDependencies[dependency];
            if (cb) {
              if (callbacks.indexOf(cb) !== -1)
                continue;
              callbacks.push(cb);
            }
          }
          for (i = 0; i < callbacks.length; i++) {
            cb = callbacks[i];
            try {
              cb(moduleOutdatedDependencies);
            } catch (err) {
              if (options.onErrored) {
                options.onErrored({
                  type: "accept-errored",
                  moduleId,
                  dependencyId: moduleOutdatedDependencies[i],
                  error: err
                });
              }
              if (!options.ignoreErrored) {
                if (!error)
                  error = err;
              }
            }
          }
        }
      }
    }
    for (i = 0; i < outdatedSelfAcceptedModules.length; i++) {
      var item = outdatedSelfAcceptedModules[i];
      moduleId = item.module;
      hotCurrentParents = item.parents;
      hotCurrentChildModule = moduleId;
      try {
        __webpack_require__(moduleId);
      } catch (err) {
        if (typeof item.errorHandler === "function") {
          try {
            item.errorHandler(err);
          } catch (err2) {
            if (options.onErrored) {
              options.onErrored({
                type: "self-accept-error-handler-errored",
                moduleId,
                error: err2,
                originalError: err
              });
            }
            if (!options.ignoreErrored) {
              if (!error)
                error = err2;
            }
            if (!error)
              error = err;
          }
        } else {
          if (options.onErrored) {
            options.onErrored({
              type: "self-accept-errored",
              moduleId,
              error: err
            });
          }
          if (!options.ignoreErrored) {
            if (!error)
              error = err;
          }
        }
      }
    }
    if (error) {
      hotSetStatus("fail");
      return Promise.reject(error);
    }
    if (hotQueuedInvalidatedModules) {
      return hotApplyInternal(options).then(function(list) {
        outdatedModules.forEach(function(moduleId) {
          if (list.indexOf(moduleId) < 0)
            list.push(moduleId);
        });
        return list;
      });
    }
    hotSetStatus("idle");
    return new Promise(function(resolve) {
      resolve(outdatedModules);
    });
  }
  function hotApplyInvalidatedModules() {
    if (hotQueuedInvalidatedModules) {
      if (!hotUpdate)
        hotUpdate = {};
      hotQueuedInvalidatedModules.forEach(hotApplyInvalidatedModule);
      hotQueuedInvalidatedModules = void 0;
      return true;
    }
  }
  function hotApplyInvalidatedModule(moduleId) {
    if (!Object.prototype.hasOwnProperty.call(hotUpdate, moduleId))
      hotUpdate[moduleId] = modules[moduleId];
  }
  var installedModules = {};
  var installedChunks = {};
  var deferredModules = [];
  function jsonpScriptSrc(chunkId) {
    return __webpack_require__.p + "static/js/" + ({}[chunkId] || chunkId) + ".chunk.js";
  }
  function __webpack_require__(moduleId) {
    if (installedModules[moduleId]) {
      return installedModules[moduleId].exports;
    }
    var module = installedModules[moduleId] = {
      i: moduleId,
      l: false,
      exports: {},
      hot: hotCreateModule(moduleId),
      parents: (hotCurrentParentsTemp = hotCurrentParents, hotCurrentParents = [], hotCurrentParentsTemp),
      children: []
    };
    __webpack_require__.$Refresh$.setup(moduleId);
    try {
      modules[moduleId].call(module.exports, module, module.exports, hotCreateRequire(moduleId));
    } finally {
      __webpack_require__.$Refresh$.cleanup(moduleId);
    }
    module.l = true;
    return module.exports;
  }
  __webpack_require__.e = function requireEnsure(chunkId) {
    var promises = [];
    var installedChunkData = installedChunks[chunkId];
    if (installedChunkData !== 0) {
      if (installedChunkData) {
        promises.push(installedChunkData[2]);
      } else {
        var promise = new Promise(function(resolve, reject) {
          installedChunkData = installedChunks[chunkId] = [resolve, reject];
        });
        promises.push(installedChunkData[2] = promise);
        var script = document.createElement("script");
        var onScriptComplete;
        script.charset = "utf-8";
        script.timeout = 120;
        if (__webpack_require__.nc) {
          script.setAttribute("nonce", __webpack_require__.nc);
        }
        script.src = jsonpScriptSrc(chunkId);
        var error = new Error();
        onScriptComplete = function(event) {
          script.onerror = script.onload = null;
          clearTimeout(timeout);
          var chunk = installedChunks[chunkId];
          if (chunk !== 0) {
            if (chunk) {
              var errorType = event && (event.type === "load" ? "missing" : event.type);
              var realSrc = event && event.target && event.target.src;
              error.message = "Loading chunk " + chunkId + " failed.\n(" + errorType + ": " + realSrc + ")";
              error.name = "ChunkLoadError";
              error.type = errorType;
              error.request = realSrc;
              chunk[1](error);
            }
            installedChunks[chunkId] = void 0;
          }
        };
        var timeout = setTimeout(function() {
          onScriptComplete({ type: "timeout", target: script });
        }, 12e4);
        script.onerror = script.onload = onScriptComplete;
        document.head.appendChild(script);
      }
    }
    return Promise.all(promises);
  };
  __webpack_require__.m = modules;
  __webpack_require__.c = installedModules;
  __webpack_require__.d = function(exports, name, getter) {
    if (!__webpack_require__.o(exports, name)) {
      Object.defineProperty(exports, name, { enumerable: true, get: getter });
    }
  };
  __webpack_require__.r = function(exports) {
    if (typeof Symbol !== "undefined" && Symbol.toStringTag) {
      Object.defineProperty(exports, Symbol.toStringTag, { value: "Module" });
    }
    Object.defineProperty(exports, "__esModule", { value: true });
  };
  __webpack_require__.t = function(value, mode) {
    if (mode & 1)
      value = __webpack_require__(value);
    if (mode & 8)
      return value;
    if (mode & 4 && typeof value === "object" && value && value.__esModule)
      return value;
    var ns = /* @__PURE__ */ Object.create(null);
    __webpack_require__.r(ns);
    Object.defineProperty(ns, "default", { enumerable: true, value });
    if (mode & 2 && typeof value != "string")
      for (var key in value)
        __webpack_require__.d(ns, key, function(key) {
          return value[key];
        }.bind(null, key));
    return ns;
  };
  __webpack_require__.n = function(module) {
    var getter = function getDefault() {
      return module && module.__esModule ? module["default"] : module;
    };
    __webpack_require__.d(getter, "a", getter);
    return getter;
  };
  __webpack_require__.o = function(object, property) {
    return Object.prototype.hasOwnProperty.call(object, property);
  };
  __webpack_require__.p = "/";
  __webpack_require__.oe = function(err) {
    console.error(err);
    throw err;
  };
  __webpack_require__.h = function() {
    return hotCurrentHash;
  };
  __webpack_require__.$Refresh$ = {
    init: function() {
      __webpack_require__.$Refresh$.cleanup = function() {
        return void 0;
      };
      __webpack_require__.$Refresh$.register = function() {
        return void 0;
      };
      __webpack_require__.$Refresh$.runtime = {};
      __webpack_require__.$Refresh$.signature = function() {
        return function(type) {
          return type;
        };
      };
    },
    setup: function(currentModuleId) {
      var prevCleanup = __webpack_require__.$Refresh$.cleanup;
      var prevReg = __webpack_require__.$Refresh$.register;
      var prevSig = __webpack_require__.$Refresh$.signature;
      __webpack_require__.$Refresh$.register = function(type, id) {
        var typeId = currentModuleId + " " + id;
        __webpack_require__.$Refresh$.runtime.register(type, typeId);
      };
      __webpack_require__.$Refresh$.signature = __webpack_require__.$Refresh$.runtime.createSignatureFunctionForTransform;
      __webpack_require__.$Refresh$.cleanup = function(cleanupModuleId) {
        if (currentModuleId === cleanupModuleId) {
          __webpack_require__.$Refresh$.register = prevReg;
          __webpack_require__.$Refresh$.signature = prevSig;
          __webpack_require__.$Refresh$.cleanup = prevCleanup;
        }
      };
    }
  };
  __webpack_require__.$Refresh$.init();
  var jsonpArray = this["webpackJsonp"] = this["webpackJsonp"] || [];
  var oldJsonpFunction = jsonpArray.push.bind(jsonpArray);
  jsonpArray.push = webpackJsonpCallback;
  jsonpArray = jsonpArray.slice();
  for (var i = 0; i < jsonpArray.length; i++)
    webpackJsonpCallback(jsonpArray[i]);
  var parentJsonpFunction = oldJsonpFunction;
  checkDeferredModules();
})({});
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
}) },[]]);
webpackJsonp.push([["0"], {"<react_refresh_runtime>": (function(module, exports, require) {
  var REACT_ELEMENT_TYPE = 60103, REACT_PORTAL_TYPE = 60106, REACT_FRAGMENT_TYPE = 60107, REACT_STRICT_MODE_TYPE = 60108, REACT_PROFILER_TYPE = 60114, REACT_PROVIDER_TYPE = 60109, REACT_CONTEXT_TYPE = 60110, REACT_FORWARD_REF_TYPE = 60112, REACT_SUSPENSE_TYPE = 60113, REACT_SUSPENSE_LIST_TYPE = 60120, REACT_MEMO_TYPE = 60115, REACT_LAZY_TYPE = 60116, REACT_SCOPE_TYPE = 60119, REACT_DEBUG_TRACING_MODE_TYPE = 60129, REACT_OFFSCREEN_TYPE = 60130, REACT_LEGACY_HIDDEN_TYPE = 60131, REACT_CACHE_TYPE = 60132;
  if (typeof Symbol == "function" && Symbol.for) {
    var symbolFor = Symbol.for;
    REACT_ELEMENT_TYPE = symbolFor("react.element"), REACT_PORTAL_TYPE = symbolFor("react.portal"), REACT_FRAGMENT_TYPE = symbolFor("react.fragment"), REACT_STRICT_MODE_TYPE = symbolFor("react.strict_mode"), REACT_PROFILER_TYPE = symbolFor("react.profiler"), REACT_PROVIDER_TYPE = symbolFor("react.provider"), REACT_CONTEXT_TYPE = symbolFor("react.context"), REACT_FORWARD_REF_TYPE = symbolFor("react.forward_ref"), REACT_SUSPENSE_TYPE = symbolFor("react.suspense"), REACT_SUSPENSE_LIST_TYPE = symbolFor("react.suspense_list"), REACT_MEMO_TYPE = symbolFor("react.memo"), REACT_LAZY_TYPE = symbolFor("react.lazy"), REACT_SCOPE_TYPE = symbolFor("react.scope"), REACT_DEBUG_TRACING_MODE_TYPE = symbolFor("react.debug_trace_mode"), REACT_OFFSCREEN_TYPE = symbolFor("react.offscreen"), REACT_LEGACY_HIDDEN_TYPE = symbolFor("react.legacy_hidden"), REACT_CACHE_TYPE = symbolFor("react.cache");
  }
  var PossiblyWeakMap = typeof WeakMap == "function" ? WeakMap : Map, allFamiliesByID = /* @__PURE__ */ new Map(), allFamiliesByType = new PossiblyWeakMap(), allSignaturesByType = new PossiblyWeakMap(), updatedFamiliesByType = new PossiblyWeakMap(), pendingUpdates = [], helpersByRendererID = /* @__PURE__ */ new Map(), helpersByRoot = /* @__PURE__ */ new Map(), mountedRoots = /* @__PURE__ */ new Set(), failedRoots = /* @__PURE__ */ new Set(), rootElements = (
    // $FlowIssue
    typeof WeakMap == "function" ? /* @__PURE__ */ new WeakMap() : null
  ), isPerformingRefresh = false;
  function computeFullKey(signature) {
    if (signature.fullKey !== null)
      return signature.fullKey;
    var fullKey = signature.ownKey, hooks;
    try {
      hooks = signature.getCustomHooks();
    } catch {
      return signature.forceReset = true, signature.fullKey = fullKey, fullKey;
    }
    for (var i = 0; i < hooks.length; i++) {
      var hook = hooks[i];
      if (typeof hook != "function")
        return signature.forceReset = true, signature.fullKey = fullKey, fullKey;
      var nestedHookSignature = allSignaturesByType.get(hook);
      if (nestedHookSignature !== void 0) {
        var nestedHookKey = computeFullKey(nestedHookSignature);
        nestedHookSignature.forceReset && (signature.forceReset = true), fullKey += "\n---\n" + nestedHookKey;
      }
    }
    return signature.fullKey = fullKey, fullKey;
  }
  function haveEqualSignatures(prevType, nextType) {
    var prevSignature = allSignaturesByType.get(prevType), nextSignature = allSignaturesByType.get(nextType);
    return prevSignature === void 0 && nextSignature === void 0 ? true : !(prevSignature === void 0 || nextSignature === void 0 || computeFullKey(prevSignature) !== computeFullKey(nextSignature) || nextSignature.forceReset);
  }
  function isReactClass(type) {
    return type.prototype && type.prototype.isReactComponent;
  }
  function canPreserveStateBetween(prevType, nextType) {
    return isReactClass(prevType) || isReactClass(nextType) ? false : !!haveEqualSignatures(prevType, nextType);
  }
  function resolveFamily(type) {
    return updatedFamiliesByType.get(type);
  }
  function cloneMap(map) {
    var clone = /* @__PURE__ */ new Map();
    return map.forEach(function(value, key) {
      clone.set(key, value);
    }), clone;
  }
  function cloneSet(set) {
    var clone = /* @__PURE__ */ new Set();
    return set.forEach(function(value) {
      clone.add(value);
    }), clone;
  }
  function getProperty(object, property) {
    try {
      return object[property];
    } catch {
      return;
    }
  }
  function performReactRefresh() {
    if (pendingUpdates.length === 0 || isPerformingRefresh)
      return null;
    isPerformingRefresh = true;
    try {
      var staleFamilies = /* @__PURE__ */ new Set(), updatedFamilies = /* @__PURE__ */ new Set(), updates = pendingUpdates;
      pendingUpdates = [], updates.forEach(function(_ref) {
        var family = _ref[0], nextType = _ref[1], prevType = family.current;
        updatedFamiliesByType.set(prevType, family), updatedFamiliesByType.set(nextType, family), family.current = nextType, canPreserveStateBetween(prevType, nextType) ? updatedFamilies.add(family) : staleFamilies.add(family);
      });
      var update = {
        updatedFamilies,
        // Families that will re-render preserving state
        staleFamilies
        // Families that will be remounted
      };
      helpersByRendererID.forEach(function(helpers) {
        helpers.setRefreshHandler(resolveFamily);
      });
      var didError = false, firstError = null, failedRootsSnapshot = cloneSet(failedRoots), mountedRootsSnapshot = cloneSet(mountedRoots), helpersByRootSnapshot = cloneMap(helpersByRoot);
      if (failedRootsSnapshot.forEach(function(root) {
        var helpers = helpersByRootSnapshot.get(root);
        if (helpers === void 0)
          throw new Error("Could not find helpers for a root. This is a bug in React Refresh.");
        if (failedRoots.has(root), rootElements !== null && rootElements.has(root)) {
          var element = rootElements.get(root);
          try {
            helpers.scheduleRoot(root, element);
          } catch (err) {
            didError || (didError = true, firstError = err);
          }
        }
      }), mountedRootsSnapshot.forEach(function(root) {
        var helpers = helpersByRootSnapshot.get(root);
        if (helpers === void 0)
          throw new Error("Could not find helpers for a root. This is a bug in React Refresh.");
        mountedRoots.has(root);
        try {
          helpers.scheduleRefresh(root, update);
        } catch (err) {
          didError || (didError = true, firstError = err);
        }
      }), didError)
        throw firstError;
      return update;
    } finally {
      isPerformingRefresh = false;
    }
  }
  function register(type, id) {
    {
      if (type === null || typeof type != "function" && typeof type != "object" || allFamiliesByType.has(type))
        return;
      var family = allFamiliesByID.get(id);
      if (family === void 0 ? (family = {
        current: type
      }, allFamiliesByID.set(id, family)) : pendingUpdates.push([family, type]), allFamiliesByType.set(type, family), typeof type == "object" && type !== null)
        switch (getProperty(type, "$$typeof")) {
          case REACT_FORWARD_REF_TYPE:
            register(type.render, id + "$render");
            break;
          case REACT_MEMO_TYPE:
            register(type.type, id + "$type");
            break;
        }
    }
  }
  function setSignature(type, key) {
    var forceReset = arguments.length > 2 && arguments[2] !== void 0 ? arguments[2] : false, getCustomHooks = arguments.length > 3 ? arguments[3] : void 0;
    if (allSignaturesByType.has(type) || allSignaturesByType.set(type, {
      forceReset,
      ownKey: key,
      fullKey: null,
      getCustomHooks: getCustomHooks || function() {
        return [];
      }
    }), typeof type == "object" && type !== null)
      switch (getProperty(type, "$$typeof")) {
        case REACT_FORWARD_REF_TYPE:
          setSignature(type.render, key, forceReset, getCustomHooks);
          break;
        case REACT_MEMO_TYPE:
          setSignature(type.type, key, forceReset, getCustomHooks);
          break;
      }
  }
  function collectCustomHooksForSignature(type) {
    {
      var signature = allSignaturesByType.get(type);
      signature !== void 0 && computeFullKey(signature);
    }
  }
  function getFamilyByID(id) {
    return allFamiliesByID.get(id);
  }
  function getFamilyByType(type) {
    return allFamiliesByType.get(type);
  }
  function findAffectedHostInstances(families) {
    {
      var affectedInstances = /* @__PURE__ */ new Set();
      return mountedRoots.forEach(function(root) {
        var helpers = helpersByRoot.get(root);
        if (helpers === void 0)
          throw new Error("Could not find helpers for a root. This is a bug in React Refresh.");
        var instancesForRoot = helpers.findHostInstancesForRefresh(root, families);
        instancesForRoot.forEach(function(inst) {
          affectedInstances.add(inst);
        });
      }), affectedInstances;
    }
  }
  function injectIntoGlobalHook(globalObject) {
    {
      var hook = globalObject.__REACT_DEVTOOLS_GLOBAL_HOOK__;
      if (hook === void 0) {
        var nextID = 0;
        globalObject.__REACT_DEVTOOLS_GLOBAL_HOOK__ = hook = {
          renderers: /* @__PURE__ */ new Map(),
          supportsFiber: true,
          inject: function(injected) {
            return nextID++;
          },
          onScheduleFiberRoot: function(id, root, children) {
          },
          onCommitFiberRoot: function(id, root, maybePriorityLevel, didError) {
          },
          onCommitFiberUnmount: function() {
          }
        };
      }
      if (hook.isDisabled) {
        console.warn("Something has shimmed the React DevTools global hook (__REACT_DEVTOOLS_GLOBAL_HOOK__). Fast Refresh is not compatible with this shim and will be disabled.");
        return;
      }
      var oldInject = hook.inject;
      hook.inject = function(injected) {
        var id = oldInject.apply(this, arguments);
        return typeof injected.scheduleRefresh == "function" && typeof injected.setRefreshHandler == "function" && helpersByRendererID.set(id, injected), id;
      }, hook.renderers.forEach(function(injected, id) {
        typeof injected.scheduleRefresh == "function" && typeof injected.setRefreshHandler == "function" && helpersByRendererID.set(id, injected);
      });
      var oldOnCommitFiberRoot = hook.onCommitFiberRoot, oldOnScheduleFiberRoot = hook.onScheduleFiberRoot || function() {
      };
      hook.onScheduleFiberRoot = function(id, root, children) {
        return isPerformingRefresh || (failedRoots.delete(root), rootElements !== null && rootElements.set(root, children)), oldOnScheduleFiberRoot.apply(this, arguments);
      }, hook.onCommitFiberRoot = function(id, root, maybePriorityLevel, didError) {
        var helpers = helpersByRendererID.get(id);
        if (helpers !== void 0) {
          helpersByRoot.set(root, helpers);
          var current = root.current, alternate = current.alternate;
          if (alternate !== null) {
            var wasMounted = alternate.memoizedState != null && alternate.memoizedState.element != null, isMounted = current.memoizedState != null && current.memoizedState.element != null;
            !wasMounted && isMounted ? (mountedRoots.add(root), failedRoots.delete(root)) : wasMounted && isMounted || (wasMounted && !isMounted ? (mountedRoots.delete(root), didError ? failedRoots.add(root) : helpersByRoot.delete(root)) : !wasMounted && !isMounted && didError && failedRoots.add(root));
          } else
            mountedRoots.add(root);
        }
        return oldOnCommitFiberRoot.apply(this, arguments);
      };
    }
  }
  function hasUnrecoverableErrors() {
    return false;
  }
  function _getMountedRootCount() {
    return mountedRoots.size;
  }
  function createSignatureFunctionForTransform() {
    {
      var savedType, hasCustomHooks, didCollectHooks = false;
      return function(type, key, forceReset, getCustomHooks) {
        if (typeof key == "string")
          return savedType || (savedType = type, hasCustomHooks = typeof getCustomHooks == "function"), type != null && (typeof type == "function" || typeof type == "object") && setSignature(type, key, forceReset, getCustomHooks), type;
        !didCollectHooks && hasCustomHooks && (didCollectHooks = true, collectCustomHooksForSignature(savedType));
      };
    }
  }
  function isLikelyComponentType(type) {
    switch (typeof type) {
      case "function": {
        if (type.prototype != null) {
          if (type.prototype.isReactComponent)
            return true;
          var ownNames = Object.getOwnPropertyNames(type.prototype);
          if (ownNames.length > 1 || ownNames[0] !== "constructor" || type.prototype.__proto__ !== Object.prototype)
            return false;
        }
        var name = type.name || type.displayName;
        return typeof name == "string" && /^[A-Z]/.test(name);
      }
      case "object": {
        if (type != null)
          switch (getProperty(type, "$$typeof")) {
            case REACT_FORWARD_REF_TYPE:
            case REACT_MEMO_TYPE:
              return true;
            default:
              return false;
          }
        return false;
      }
      default:
        return false;
    }
  }
  exports._getMountedRootCount = _getMountedRootCount, exports.collectCustomHooksForSignature = collectCustomHooksForSignature, exports.createSignatureFunctionForTransform = createSignatureFunctionForTransform, exports.findAffectedHostInstances = findAffectedHostInstances, exports.getFamilyByID = getFamilyByID, exports.getFamilyByType = getFamilyByType, exports.hasUnrecoverableErrors = hasUnrecoverableErrors, exports.injectIntoGlobalHook = injectIntoGlobalHook, exports.isLikelyComponentType = isLikelyComponentType, exports.performReactRefresh = performReactRefresh, exports.register = register, exports.setSignature = setSignature, exports.executeRuntime = executeRuntime, exports.getModuleExports = getModuleExports;
  const Refresh = exports;
  function getModuleExports(moduleId) {
    if (typeof moduleId > "u")
      return {};
    var maybeModule = require.c[moduleId];
    if (typeof maybeModule > "u")
      return console.warn("[React Refresh] Failed to get exports for module: " + moduleId + "."), {};
    var exportsOrPromise = maybeModule.exports;
    return typeof Promise < "u" && exportsOrPromise instanceof Promise ? exportsOrPromise.then(function(exports) {
      return exports;
    }) : exportsOrPromise;
  }
  function getReactRefreshBoundarySignature(moduleExports) {
    var signature = [];
    if (signature.push(Refresh.getFamilyByType(moduleExports)), moduleExports == null || typeof moduleExports != "object")
      return signature;
    for (var key in moduleExports)
      key !== "__esModule" && (signature.push(key), signature.push(Refresh.getFamilyByType(moduleExports[key])));
    return signature;
  }
  function createDebounceUpdate() {
    var refreshTimeout;
    function enqueueUpdate(callback) {
      typeof refreshTimeout > "u" && (refreshTimeout = setTimeout(function() {
        refreshTimeout = void 0, Refresh.performReactRefresh(), callback();
      }, 30));
    }
    return enqueueUpdate;
  }
  function isReactRefreshBoundary(moduleExports) {
    if (Refresh.isLikelyComponentType(moduleExports))
      return true;
    if (moduleExports == null || typeof moduleExports != "object")
      return false;
    var hasExports = false, areAllExportsComponents = true;
    for (var key in moduleExports)
      if (hasExports = true, key !== "__esModule") {
        var exportValue = moduleExports[key];
        Refresh.isLikelyComponentType(exportValue) || (areAllExportsComponents = false);
      }
    return hasExports && areAllExportsComponents;
  }
  function registerExportsForReactRefresh(moduleExports, moduleId) {
    if (Refresh.isLikelyComponentType(moduleExports) && Refresh.register(moduleExports, moduleId + " %exports%"), !(moduleExports == null || typeof moduleExports != "object")) {
      for (var key in moduleExports)
        if (key !== "__esModule") {
          var exportValue = moduleExports[key];
          if (Refresh.isLikelyComponentType(exportValue)) {
            var typeID = moduleId + " %exports% " + key;
            Refresh.register(exportValue, typeID);
          }
        }
    }
  }
  function shouldInvalidateReactRefreshBoundary(prevExports, nextExports) {
    var prevSignature = getReactRefreshBoundarySignature(prevExports), nextSignature = getReactRefreshBoundarySignature(nextExports);
    if (prevSignature.length !== nextSignature.length)
      return true;
    for (var i = 0; i < nextSignature.length; i += 1)
      if (prevSignature[i] !== nextSignature[i])
        return true;
    return false;
  }
  var enqueueUpdate = createDebounceUpdate();
  function executeRuntime(moduleExports, moduleId, webpackHot, refreshOverlay, isTest) {
    if (registerExportsForReactRefresh(moduleExports, moduleId), webpackHot) {
      var isHotUpdate = !!webpackHot.data, prevExports;
      isHotUpdate && (prevExports = webpackHot.data.prevExports), isReactRefreshBoundary(moduleExports) ? (webpackHot.dispose(
        /**
         * A callback to performs a full refresh if React has unrecoverable errors,
         * and also caches the to-be-disposed module.
         * @param {*} data A hot module data object from Webpack HMR.
         * @returns {void}
         */
        function(data) {
          data.prevExports = moduleExports;
        }
      ), webpackHot.accept(
        /**
         * An error handler to allow self-recovering behaviours.
         * @param {Error} error An error occurred during evaluation of a module.
         * @returns {void}
         */
        function hotErrorHandler(error) {
          typeof refreshOverlay < "u" && refreshOverlay && refreshOverlay.handleRuntimeError(error), typeof isTest < "u" && isTest && window.onHotAcceptError && window.onHotAcceptError(error.message), require.c[moduleId].hot.accept(hotErrorHandler);
        }
      ), isHotUpdate && (isReactRefreshBoundary(prevExports) && shouldInvalidateReactRefreshBoundary(prevExports, moduleExports) ? webpackHot.invalidate() : enqueueUpdate(
        /**
         * A function to dismiss the error overlay after performing React refresh.
         * @returns {void}
         */
        function() {
          typeof refreshOverlay < "u" && refreshOverlay && refreshOverlay.clearRuntimeErrors();
        }
      ))) : isHotUpdate && typeof prevExports < "u" && webpackHot.invalidate();
    }
  }
  injectIntoGlobalHook(function() {
    return this;
  }()), require.$Refresh$.runtime = exports;
}) },[]]);
webpackJsonp.push([["0"], {"/Users/yanbo.wu/Documents/projects/tmp/esbuild/test/sourcemap/app/node_modules/react/cjs/react.development.js": (function(module, exports, require) {
  "use strict";
  /**
   * @license React
   * react.development.js
   *
   * Copyright (c) Facebook, Inc. and its affiliates.
   *
   * This source code is licensed under the MIT license found in the
   * LICENSE file in the root directory of this source tree.
   */
  (function() {
    "use strict";
    typeof __REACT_DEVTOOLS_GLOBAL_HOOK__ != "undefined" && typeof __REACT_DEVTOOLS_GLOBAL_HOOK__.registerInternalModuleStart == "function" && __REACT_DEVTOOLS_GLOBAL_HOOK__.registerInternalModuleStart(new Error());
    var ReactVersion = "18.2.0", REACT_ELEMENT_TYPE = Symbol.for("react.element"), REACT_PORTAL_TYPE = Symbol.for("react.portal"), REACT_FRAGMENT_TYPE = Symbol.for("react.fragment"), REACT_STRICT_MODE_TYPE = Symbol.for("react.strict_mode"), REACT_PROFILER_TYPE = Symbol.for("react.profiler"), REACT_PROVIDER_TYPE = Symbol.for("react.provider"), REACT_CONTEXT_TYPE = Symbol.for("react.context"), REACT_FORWARD_REF_TYPE = Symbol.for("react.forward_ref"), REACT_SUSPENSE_TYPE = Symbol.for("react.suspense"), REACT_SUSPENSE_LIST_TYPE = Symbol.for("react.suspense_list"), REACT_MEMO_TYPE = Symbol.for("react.memo"), REACT_LAZY_TYPE = Symbol.for("react.lazy"), REACT_OFFSCREEN_TYPE = Symbol.for("react.offscreen"), MAYBE_ITERATOR_SYMBOL = Symbol.iterator, FAUX_ITERATOR_SYMBOL = "@@iterator";
    function getIteratorFn(maybeIterable) {
      if (maybeIterable === null || typeof maybeIterable != "object")
        return null;
      var maybeIterator = MAYBE_ITERATOR_SYMBOL && maybeIterable[MAYBE_ITERATOR_SYMBOL] || maybeIterable[FAUX_ITERATOR_SYMBOL];
      return typeof maybeIterator == "function" ? maybeIterator : null;
    }
    var ReactCurrentDispatcher = {
      current: null
    }, ReactCurrentBatchConfig = {
      transition: null
    }, ReactCurrentActQueue = {
      current: null,
      isBatchingLegacy: false,
      didScheduleLegacyUpdate: false
    }, ReactCurrentOwner = {
      current: null
    }, ReactDebugCurrentFrame = {}, currentExtraStackFrame = null;
    function setExtraStackFrame(stack) {
      currentExtraStackFrame = stack;
    }
    ReactDebugCurrentFrame.setExtraStackFrame = function(stack) {
      currentExtraStackFrame = stack;
    }, ReactDebugCurrentFrame.getCurrentStack = null, ReactDebugCurrentFrame.getStackAddendum = function() {
      var stack = "";
      currentExtraStackFrame && (stack += currentExtraStackFrame);
      var impl = ReactDebugCurrentFrame.getCurrentStack;
      return impl && (stack += impl() || ""), stack;
    };
    var enableScopeAPI = false, enableCacheElement = false, enableTransitionTracing = false, enableLegacyHidden = false, enableDebugTracing = false, ReactSharedInternals = {
      ReactCurrentDispatcher,
      ReactCurrentBatchConfig,
      ReactCurrentOwner
    };
    ReactSharedInternals.ReactDebugCurrentFrame = ReactDebugCurrentFrame, ReactSharedInternals.ReactCurrentActQueue = ReactCurrentActQueue;
    function warn(format) {
      {
        for (var _len = arguments.length, args = new Array(_len > 1 ? _len - 1 : 0), _key = 1; _key < _len; _key++)
          args[_key - 1] = arguments[_key];
        printWarning("warn", format, args);
      }
    }
    function error(format) {
      {
        for (var _len2 = arguments.length, args = new Array(_len2 > 1 ? _len2 - 1 : 0), _key2 = 1; _key2 < _len2; _key2++)
          args[_key2 - 1] = arguments[_key2];
        printWarning("error", format, args);
      }
    }
    function printWarning(level, format, args) {
      {
        var ReactDebugCurrentFrame = ReactSharedInternals.ReactDebugCurrentFrame, stack = ReactDebugCurrentFrame.getStackAddendum();
        stack !== "" && (format += "%s", args = args.concat([stack]));
        var argsWithFormat = args.map(function(item) {
          return String(item);
        });
        argsWithFormat.unshift("Warning: " + format), Function.prototype.apply.call(console[level], console, argsWithFormat);
      }
    }
    var didWarnStateUpdateForUnmountedComponent = {};
    function warnNoop(publicInstance, callerName) {
      {
        var _constructor = publicInstance.constructor, componentName = _constructor && (_constructor.displayName || _constructor.name) || "ReactClass", warningKey = componentName + "." + callerName;
        if (didWarnStateUpdateForUnmountedComponent[warningKey])
          return;
        error("Can't call %s on a component that is not yet mounted. This is a no-op, but it might indicate a bug in your application. Instead, assign to `this.state` directly or define a `state = {};` class property with the desired state in the %s component.", callerName, componentName), didWarnStateUpdateForUnmountedComponent[warningKey] = true;
      }
    }
    var ReactNoopUpdateQueue = {
      isMounted: function(publicInstance) {
        return false;
      },
      enqueueForceUpdate: function(publicInstance, callback, callerName) {
        warnNoop(publicInstance, "forceUpdate");
      },
      enqueueReplaceState: function(publicInstance, completeState, callback, callerName) {
        warnNoop(publicInstance, "replaceState");
      },
      enqueueSetState: function(publicInstance, partialState, callback, callerName) {
        warnNoop(publicInstance, "setState");
      }
    }, assign = Object.assign, emptyObject = {};
    Object.freeze(emptyObject);
    function Component(props, context, updater) {
      this.props = props, this.context = context, this.refs = emptyObject, this.updater = updater || ReactNoopUpdateQueue;
    }
    Component.prototype.isReactComponent = {}, Component.prototype.setState = function(partialState, callback) {
      if (typeof partialState != "object" && typeof partialState != "function" && partialState != null)
        throw new Error("setState(...): takes an object of state variables to update or a function which returns an object of state variables.");
      this.updater.enqueueSetState(this, partialState, callback, "setState");
    }, Component.prototype.forceUpdate = function(callback) {
      this.updater.enqueueForceUpdate(this, callback, "forceUpdate");
    };
    {
      var deprecatedAPIs = {
        isMounted: ["isMounted", "Instead, make sure to clean up subscriptions and pending requests in componentWillUnmount to prevent memory leaks."],
        replaceState: ["replaceState", "Refactor your code to use setState instead (see https://github.com/facebook/react/issues/3236)."]
      }, defineDeprecationWarning = function(methodName, info) {
        Object.defineProperty(Component.prototype, methodName, {
          get: function() {
            warn("%s(...) is deprecated in plain JavaScript React classes. %s", info[0], info[1]);
          }
        });
      };
      for (var fnName in deprecatedAPIs)
        deprecatedAPIs.hasOwnProperty(fnName) && defineDeprecationWarning(fnName, deprecatedAPIs[fnName]);
    }
    function ComponentDummy() {
    }
    ComponentDummy.prototype = Component.prototype;
    function PureComponent(props, context, updater) {
      this.props = props, this.context = context, this.refs = emptyObject, this.updater = updater || ReactNoopUpdateQueue;
    }
    var pureComponentPrototype = PureComponent.prototype = new ComponentDummy();
    pureComponentPrototype.constructor = PureComponent, assign(pureComponentPrototype, Component.prototype), pureComponentPrototype.isPureReactComponent = true;
    function createRef() {
      var refObject = {
        current: null
      };
      return Object.seal(refObject), refObject;
    }
    var isArrayImpl = Array.isArray;
    function isArray(a) {
      return isArrayImpl(a);
    }
    function typeName(value) {
      {
        var hasToStringTag = typeof Symbol == "function" && Symbol.toStringTag, type = hasToStringTag && value[Symbol.toStringTag] || value.constructor.name || "Object";
        return type;
      }
    }
    function willCoercionThrow(value) {
      try {
        return testStringCoercion(value), false;
      } catch (e) {
        return true;
      }
    }
    function testStringCoercion(value) {
      return "" + value;
    }
    function checkKeyStringCoercion(value) {
      if (willCoercionThrow(value))
        return error("The provided key is an unsupported type %s. This value must be coerced to a string before before using it here.", typeName(value)), testStringCoercion(value);
    }
    function getWrappedName(outerType, innerType, wrapperName) {
      var displayName = outerType.displayName;
      if (displayName)
        return displayName;
      var functionName = innerType.displayName || innerType.name || "";
      return functionName !== "" ? wrapperName + "(" + functionName + ")" : wrapperName;
    }
    function getContextName(type) {
      return type.displayName || "Context";
    }
    function getComponentNameFromType(type) {
      if (type == null)
        return null;
      if (typeof type.tag == "number" && error("Received an unexpected object in getComponentNameFromType(). This is likely a bug in React. Please file an issue."), typeof type == "function")
        return type.displayName || type.name || null;
      if (typeof type == "string")
        return type;
      switch (type) {
        case REACT_FRAGMENT_TYPE:
          return "Fragment";
        case REACT_PORTAL_TYPE:
          return "Portal";
        case REACT_PROFILER_TYPE:
          return "Profiler";
        case REACT_STRICT_MODE_TYPE:
          return "StrictMode";
        case REACT_SUSPENSE_TYPE:
          return "Suspense";
        case REACT_SUSPENSE_LIST_TYPE:
          return "SuspenseList";
      }
      if (typeof type == "object")
        switch (type.$$typeof) {
          case REACT_CONTEXT_TYPE:
            var context = type;
            return getContextName(context) + ".Consumer";
          case REACT_PROVIDER_TYPE:
            var provider = type;
            return getContextName(provider._context) + ".Provider";
          case REACT_FORWARD_REF_TYPE:
            return getWrappedName(type, type.render, "ForwardRef");
          case REACT_MEMO_TYPE:
            var outerName = type.displayName || null;
            return outerName !== null ? outerName : getComponentNameFromType(type.type) || "Memo";
          case REACT_LAZY_TYPE: {
            var lazyComponent = type, payload = lazyComponent._payload, init = lazyComponent._init;
            try {
              return getComponentNameFromType(init(payload));
            } catch (x) {
              return null;
            }
          }
        }
      return null;
    }
    var hasOwnProperty = Object.prototype.hasOwnProperty, RESERVED_PROPS = {
      key: true,
      ref: true,
      __self: true,
      __source: true
    }, specialPropKeyWarningShown, specialPropRefWarningShown, didWarnAboutStringRefs;
    didWarnAboutStringRefs = {};
    function hasValidRef(config) {
      if (hasOwnProperty.call(config, "ref")) {
        var getter = Object.getOwnPropertyDescriptor(config, "ref").get;
        if (getter && getter.isReactWarning)
          return false;
      }
      return config.ref !== void 0;
    }
    function hasValidKey(config) {
      if (hasOwnProperty.call(config, "key")) {
        var getter = Object.getOwnPropertyDescriptor(config, "key").get;
        if (getter && getter.isReactWarning)
          return false;
      }
      return config.key !== void 0;
    }
    function defineKeyPropWarningGetter(props, displayName) {
      var warnAboutAccessingKey = function() {
        specialPropKeyWarningShown || (specialPropKeyWarningShown = true, error("%s: `key` is not a prop. Trying to access it will result in `undefined` being returned. If you need to access the same value within the child component, you should pass it as a different prop. (https://reactjs.org/link/special-props)", displayName));
      };
      warnAboutAccessingKey.isReactWarning = true, Object.defineProperty(props, "key", {
        get: warnAboutAccessingKey,
        configurable: true
      });
    }
    function defineRefPropWarningGetter(props, displayName) {
      var warnAboutAccessingRef = function() {
        specialPropRefWarningShown || (specialPropRefWarningShown = true, error("%s: `ref` is not a prop. Trying to access it will result in `undefined` being returned. If you need to access the same value within the child component, you should pass it as a different prop. (https://reactjs.org/link/special-props)", displayName));
      };
      warnAboutAccessingRef.isReactWarning = true, Object.defineProperty(props, "ref", {
        get: warnAboutAccessingRef,
        configurable: true
      });
    }
    function warnIfStringRefCannotBeAutoConverted(config) {
      if (typeof config.ref == "string" && ReactCurrentOwner.current && config.__self && ReactCurrentOwner.current.stateNode !== config.__self) {
        var componentName = getComponentNameFromType(ReactCurrentOwner.current.type);
        didWarnAboutStringRefs[componentName] || (error('Component "%s" contains the string ref "%s". Support for string refs will be removed in a future major release. This case cannot be automatically converted to an arrow function. We ask you to manually fix this case by using useRef() or createRef() instead. Learn more about using refs safely here: https://reactjs.org/link/strict-mode-string-ref', componentName, config.ref), didWarnAboutStringRefs[componentName] = true);
      }
    }
    var ReactElement = function(type, key, ref, self, source, owner, props) {
      var element = {
        $$typeof: REACT_ELEMENT_TYPE,
        type,
        key,
        ref,
        props,
        _owner: owner
      };
      return element._store = {}, Object.defineProperty(element._store, "validated", {
        configurable: false,
        enumerable: false,
        writable: true,
        value: false
      }), Object.defineProperty(element, "_self", {
        configurable: false,
        enumerable: false,
        writable: false,
        value: self
      }), Object.defineProperty(element, "_source", {
        configurable: false,
        enumerable: false,
        writable: false,
        value: source
      }), Object.freeze && (Object.freeze(element.props), Object.freeze(element)), element;
    };
    function createElement(type, config, children) {
      var propName, props = {}, key = null, ref = null, self = null, source = null;
      if (config != null) {
        hasValidRef(config) && (ref = config.ref, warnIfStringRefCannotBeAutoConverted(config)), hasValidKey(config) && (checkKeyStringCoercion(config.key), key = "" + config.key), self = config.__self === void 0 ? null : config.__self, source = config.__source === void 0 ? null : config.__source;
        for (propName in config)
          hasOwnProperty.call(config, propName) && !RESERVED_PROPS.hasOwnProperty(propName) && (props[propName] = config[propName]);
      }
      var childrenLength = arguments.length - 2;
      if (childrenLength === 1)
        props.children = children;
      else if (childrenLength > 1) {
        for (var childArray = Array(childrenLength), i = 0; i < childrenLength; i++)
          childArray[i] = arguments[i + 2];
        Object.freeze && Object.freeze(childArray), props.children = childArray;
      }
      if (type && type.defaultProps) {
        var defaultProps = type.defaultProps;
        for (propName in defaultProps)
          props[propName] === void 0 && (props[propName] = defaultProps[propName]);
      }
      if (key || ref) {
        var displayName = typeof type == "function" ? type.displayName || type.name || "Unknown" : type;
        key && defineKeyPropWarningGetter(props, displayName), ref && defineRefPropWarningGetter(props, displayName);
      }
      return ReactElement(type, key, ref, self, source, ReactCurrentOwner.current, props);
    }
    function cloneAndReplaceKey(oldElement, newKey) {
      var newElement = ReactElement(oldElement.type, newKey, oldElement.ref, oldElement._self, oldElement._source, oldElement._owner, oldElement.props);
      return newElement;
    }
    function cloneElement(element, config, children) {
      if (element == null)
        throw new Error("React.cloneElement(...): The argument must be a React element, but you passed " + element + ".");
      var propName, props = assign({}, element.props), key = element.key, ref = element.ref, self = element._self, source = element._source, owner = element._owner;
      if (config != null) {
        hasValidRef(config) && (ref = config.ref, owner = ReactCurrentOwner.current), hasValidKey(config) && (checkKeyStringCoercion(config.key), key = "" + config.key);
        var defaultProps;
        element.type && element.type.defaultProps && (defaultProps = element.type.defaultProps);
        for (propName in config)
          hasOwnProperty.call(config, propName) && !RESERVED_PROPS.hasOwnProperty(propName) && (config[propName] === void 0 && defaultProps !== void 0 ? props[propName] = defaultProps[propName] : props[propName] = config[propName]);
      }
      var childrenLength = arguments.length - 2;
      if (childrenLength === 1)
        props.children = children;
      else if (childrenLength > 1) {
        for (var childArray = Array(childrenLength), i = 0; i < childrenLength; i++)
          childArray[i] = arguments[i + 2];
        props.children = childArray;
      }
      return ReactElement(element.type, key, ref, self, source, owner, props);
    }
    function isValidElement(object) {
      return typeof object == "object" && object !== null && object.$$typeof === REACT_ELEMENT_TYPE;
    }
    var SEPARATOR = ".", SUBSEPARATOR = ":";
    function escape(key) {
      var escapeRegex = /[=:]/g, escaperLookup = {
        "=": "=0",
        ":": "=2"
      }, escapedString = key.replace(escapeRegex, function(match) {
        return escaperLookup[match];
      });
      return "$" + escapedString;
    }
    var didWarnAboutMaps = false, userProvidedKeyEscapeRegex = /\/+/g;
    function escapeUserProvidedKey(text) {
      return text.replace(userProvidedKeyEscapeRegex, "$&/");
    }
    function getElementKey(element, index) {
      return typeof element == "object" && element !== null && element.key != null ? (checkKeyStringCoercion(element.key), escape("" + element.key)) : index.toString(36);
    }
    function mapIntoArray(children, array, escapedPrefix, nameSoFar, callback) {
      var type = typeof children;
      (type === "undefined" || type === "boolean") && (children = null);
      var invokeCallback = false;
      if (children === null)
        invokeCallback = true;
      else
        switch (type) {
          case "string":
          case "number":
            invokeCallback = true;
            break;
          case "object":
            switch (children.$$typeof) {
              case REACT_ELEMENT_TYPE:
              case REACT_PORTAL_TYPE:
                invokeCallback = true;
            }
        }
      if (invokeCallback) {
        var _child = children, mappedChild = callback(_child), childKey = nameSoFar === "" ? SEPARATOR + getElementKey(_child, 0) : nameSoFar;
        if (isArray(mappedChild)) {
          var escapedChildKey = "";
          childKey != null && (escapedChildKey = escapeUserProvidedKey(childKey) + "/"), mapIntoArray(mappedChild, array, escapedChildKey, "", function(c) {
            return c;
          });
        } else
          mappedChild != null && (isValidElement(mappedChild) && (mappedChild.key && (!_child || _child.key !== mappedChild.key) && checkKeyStringCoercion(mappedChild.key), mappedChild = cloneAndReplaceKey(
            mappedChild,
            escapedPrefix + (mappedChild.key && (!_child || _child.key !== mappedChild.key) ? escapeUserProvidedKey("" + mappedChild.key) + "/" : "") + childKey
          )), array.push(mappedChild));
        return 1;
      }
      var child, nextName, subtreeCount = 0, nextNamePrefix = nameSoFar === "" ? SEPARATOR : nameSoFar + SUBSEPARATOR;
      if (isArray(children))
        for (var i = 0; i < children.length; i++)
          child = children[i], nextName = nextNamePrefix + getElementKey(child, i), subtreeCount += mapIntoArray(child, array, escapedPrefix, nextName, callback);
      else {
        var iteratorFn = getIteratorFn(children);
        if (typeof iteratorFn == "function") {
          var iterableChildren = children;
          iteratorFn === iterableChildren.entries && (didWarnAboutMaps || warn("Using Maps as children is not supported. Use an array of keyed ReactElements instead."), didWarnAboutMaps = true);
          for (var iterator = iteratorFn.call(iterableChildren), step, ii = 0; !(step = iterator.next()).done; )
            child = step.value, nextName = nextNamePrefix + getElementKey(child, ii++), subtreeCount += mapIntoArray(child, array, escapedPrefix, nextName, callback);
        } else if (type === "object") {
          var childrenString = String(children);
          throw new Error("Objects are not valid as a React child (found: " + (childrenString === "[object Object]" ? "object with keys {" + Object.keys(children).join(", ") + "}" : childrenString) + "). If you meant to render a collection of children, use an array instead.");
        }
      }
      return subtreeCount;
    }
    function mapChildren(children, func, context) {
      if (children == null)
        return children;
      var result = [], count = 0;
      return mapIntoArray(children, result, "", "", function(child) {
        return func.call(context, child, count++);
      }), result;
    }
    function countChildren(children) {
      var n = 0;
      return mapChildren(children, function() {
        n++;
      }), n;
    }
    function forEachChildren(children, forEachFunc, forEachContext) {
      mapChildren(children, function() {
        forEachFunc.apply(this, arguments);
      }, forEachContext);
    }
    function toArray(children) {
      return mapChildren(children, function(child) {
        return child;
      }) || [];
    }
    function onlyChild(children) {
      if (!isValidElement(children))
        throw new Error("React.Children.only expected to receive a single React element child.");
      return children;
    }
    function createContext(defaultValue) {
      var context = {
        $$typeof: REACT_CONTEXT_TYPE,
        _currentValue: defaultValue,
        _currentValue2: defaultValue,
        _threadCount: 0,
        Provider: null,
        Consumer: null,
        _defaultValue: null,
        _globalName: null
      };
      context.Provider = {
        $$typeof: REACT_PROVIDER_TYPE,
        _context: context
      };
      var hasWarnedAboutUsingNestedContextConsumers = false, hasWarnedAboutUsingConsumerProvider = false, hasWarnedAboutDisplayNameOnConsumer = false;
      {
        var Consumer = {
          $$typeof: REACT_CONTEXT_TYPE,
          _context: context
        };
        Object.defineProperties(Consumer, {
          Provider: {
            get: function() {
              return hasWarnedAboutUsingConsumerProvider || (hasWarnedAboutUsingConsumerProvider = true, error("Rendering <Context.Consumer.Provider> is not supported and will be removed in a future major release. Did you mean to render <Context.Provider> instead?")), context.Provider;
            },
            set: function(_Provider) {
              context.Provider = _Provider;
            }
          },
          _currentValue: {
            get: function() {
              return context._currentValue;
            },
            set: function(_currentValue) {
              context._currentValue = _currentValue;
            }
          },
          _currentValue2: {
            get: function() {
              return context._currentValue2;
            },
            set: function(_currentValue2) {
              context._currentValue2 = _currentValue2;
            }
          },
          _threadCount: {
            get: function() {
              return context._threadCount;
            },
            set: function(_threadCount) {
              context._threadCount = _threadCount;
            }
          },
          Consumer: {
            get: function() {
              return hasWarnedAboutUsingNestedContextConsumers || (hasWarnedAboutUsingNestedContextConsumers = true, error("Rendering <Context.Consumer.Consumer> is not supported and will be removed in a future major release. Did you mean to render <Context.Consumer> instead?")), context.Consumer;
            }
          },
          displayName: {
            get: function() {
              return context.displayName;
            },
            set: function(displayName) {
              hasWarnedAboutDisplayNameOnConsumer || (warn("Setting `displayName` on Context.Consumer has no effect. You should set it directly on the context with Context.displayName = '%s'.", displayName), hasWarnedAboutDisplayNameOnConsumer = true);
            }
          }
        }), context.Consumer = Consumer;
      }
      return context._currentRenderer = null, context._currentRenderer2 = null, context;
    }
    var Uninitialized = -1, Pending = 0, Resolved = 1, Rejected = 2;
    function lazyInitializer(payload) {
      if (payload._status === Uninitialized) {
        var ctor = payload._result, thenable = ctor();
        if (thenable.then(function(moduleObject) {
          if (payload._status === Pending || payload._status === Uninitialized) {
            var resolved = payload;
            resolved._status = Resolved, resolved._result = moduleObject;
          }
        }, function(error) {
          if (payload._status === Pending || payload._status === Uninitialized) {
            var rejected = payload;
            rejected._status = Rejected, rejected._result = error;
          }
        }), payload._status === Uninitialized) {
          var pending = payload;
          pending._status = Pending, pending._result = thenable;
        }
      }
      if (payload._status === Resolved) {
        var moduleObject = payload._result;
        return moduleObject === void 0 && error("lazy: Expected the result of a dynamic import() call. Instead received: %s\n\nYour code should look like: \n  const MyComponent = lazy(() => import('./MyComponent'))\n\nDid you accidentally put curly braces around the import?", moduleObject), "default" in moduleObject || error("lazy: Expected the result of a dynamic import() call. Instead received: %s\n\nYour code should look like: \n  const MyComponent = lazy(() => import('./MyComponent'))", moduleObject), moduleObject.default;
      } else
        throw payload._result;
    }
    function lazy(ctor) {
      var payload = {
        _status: Uninitialized,
        _result: ctor
      }, lazyType = {
        $$typeof: REACT_LAZY_TYPE,
        _payload: payload,
        _init: lazyInitializer
      };
      {
        var defaultProps, propTypes;
        Object.defineProperties(lazyType, {
          defaultProps: {
            configurable: true,
            get: function() {
              return defaultProps;
            },
            set: function(newDefaultProps) {
              error("React.lazy(...): It is not supported to assign `defaultProps` to a lazy component import. Either specify them where the component is defined, or create a wrapping component around it."), defaultProps = newDefaultProps, Object.defineProperty(lazyType, "defaultProps", {
                enumerable: true
              });
            }
          },
          propTypes: {
            configurable: true,
            get: function() {
              return propTypes;
            },
            set: function(newPropTypes) {
              error("React.lazy(...): It is not supported to assign `propTypes` to a lazy component import. Either specify them where the component is defined, or create a wrapping component around it."), propTypes = newPropTypes, Object.defineProperty(lazyType, "propTypes", {
                enumerable: true
              });
            }
          }
        });
      }
      return lazyType;
    }
    function forwardRef(render) {
      render != null && render.$$typeof === REACT_MEMO_TYPE ? error("forwardRef requires a render function but received a `memo` component. Instead of forwardRef(memo(...)), use memo(forwardRef(...)).") : typeof render != "function" ? error("forwardRef requires a render function but was given %s.", render === null ? "null" : typeof render) : render.length !== 0 && render.length !== 2 && error("forwardRef render functions accept exactly two parameters: props and ref. %s", render.length === 1 ? "Did you forget to use the ref parameter?" : "Any additional parameter will be undefined."), render != null && (render.defaultProps != null || render.propTypes != null) && error("forwardRef render functions do not support propTypes or defaultProps. Did you accidentally pass a React component?");
      var elementType = {
        $$typeof: REACT_FORWARD_REF_TYPE,
        render
      };
      {
        var ownName;
        Object.defineProperty(elementType, "displayName", {
          enumerable: false,
          configurable: true,
          get: function() {
            return ownName;
          },
          set: function(name) {
            ownName = name, !render.name && !render.displayName && (render.displayName = name);
          }
        });
      }
      return elementType;
    }
    var REACT_MODULE_REFERENCE;
    REACT_MODULE_REFERENCE = Symbol.for("react.module.reference");
    function isValidElementType(type) {
      return !!(typeof type == "string" || typeof type == "function" || type === REACT_FRAGMENT_TYPE || type === REACT_PROFILER_TYPE || enableDebugTracing || type === REACT_STRICT_MODE_TYPE || type === REACT_SUSPENSE_TYPE || type === REACT_SUSPENSE_LIST_TYPE || enableLegacyHidden || type === REACT_OFFSCREEN_TYPE || enableScopeAPI || enableCacheElement || enableTransitionTracing || typeof type == "object" && type !== null && (type.$$typeof === REACT_LAZY_TYPE || type.$$typeof === REACT_MEMO_TYPE || type.$$typeof === REACT_PROVIDER_TYPE || type.$$typeof === REACT_CONTEXT_TYPE || type.$$typeof === REACT_FORWARD_REF_TYPE || type.$$typeof === REACT_MODULE_REFERENCE || type.getModuleId !== void 0));
    }
    function memo(type, compare) {
      isValidElementType(type) || error("memo: The first argument must be a component. Instead received: %s", type === null ? "null" : typeof type);
      var elementType = {
        $$typeof: REACT_MEMO_TYPE,
        type,
        compare: compare === void 0 ? null : compare
      };
      {
        var ownName;
        Object.defineProperty(elementType, "displayName", {
          enumerable: false,
          configurable: true,
          get: function() {
            return ownName;
          },
          set: function(name) {
            ownName = name, !type.name && !type.displayName && (type.displayName = name);
          }
        });
      }
      return elementType;
    }
    function resolveDispatcher() {
      var dispatcher = ReactCurrentDispatcher.current;
      return dispatcher === null && error("Invalid hook call. Hooks can only be called inside of the body of a function component. This could happen for one of the following reasons:\n1. You might have mismatching versions of React and the renderer (such as React DOM)\n2. You might be breaking the Rules of Hooks\n3. You might have more than one copy of React in the same app\nSee https://reactjs.org/link/invalid-hook-call for tips about how to debug and fix this problem."), dispatcher;
    }
    function useContext(Context) {
      var dispatcher = resolveDispatcher();
      if (Context._context !== void 0) {
        var realContext = Context._context;
        realContext.Consumer === Context ? error("Calling useContext(Context.Consumer) is not supported, may cause bugs, and will be removed in a future major release. Did you mean to call useContext(Context) instead?") : realContext.Provider === Context && error("Calling useContext(Context.Provider) is not supported. Did you mean to call useContext(Context) instead?");
      }
      return dispatcher.useContext(Context);
    }
    function useState(initialState) {
      var dispatcher = resolveDispatcher();
      return dispatcher.useState(initialState);
    }
    function useReducer(reducer, initialArg, init) {
      var dispatcher = resolveDispatcher();
      return dispatcher.useReducer(reducer, initialArg, init);
    }
    function useRef(initialValue) {
      var dispatcher = resolveDispatcher();
      return dispatcher.useRef(initialValue);
    }
    function useEffect(create, deps) {
      var dispatcher = resolveDispatcher();
      return dispatcher.useEffect(create, deps);
    }
    function useInsertionEffect(create, deps) {
      var dispatcher = resolveDispatcher();
      return dispatcher.useInsertionEffect(create, deps);
    }
    function useLayoutEffect(create, deps) {
      var dispatcher = resolveDispatcher();
      return dispatcher.useLayoutEffect(create, deps);
    }
    function useCallback(callback, deps) {
      var dispatcher = resolveDispatcher();
      return dispatcher.useCallback(callback, deps);
    }
    function useMemo(create, deps) {
      var dispatcher = resolveDispatcher();
      return dispatcher.useMemo(create, deps);
    }
    function useImperativeHandle(ref, create, deps) {
      var dispatcher = resolveDispatcher();
      return dispatcher.useImperativeHandle(ref, create, deps);
    }
    function useDebugValue(value, formatterFn) {
      {
        var dispatcher = resolveDispatcher();
        return dispatcher.useDebugValue(value, formatterFn);
      }
    }
    function useTransition() {
      var dispatcher = resolveDispatcher();
      return dispatcher.useTransition();
    }
    function useDeferredValue(value) {
      var dispatcher = resolveDispatcher();
      return dispatcher.useDeferredValue(value);
    }
    function useId() {
      var dispatcher = resolveDispatcher();
      return dispatcher.useId();
    }
    function useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot) {
      var dispatcher = resolveDispatcher();
      return dispatcher.useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
    }
    var disabledDepth = 0, prevLog, prevInfo, prevWarn, prevError, prevGroup, prevGroupCollapsed, prevGroupEnd;
    function disabledLog() {
    }
    disabledLog.__reactDisabledLog = true;
    function disableLogs() {
      {
        if (disabledDepth === 0) {
          prevLog = console.log, prevInfo = console.info, prevWarn = console.warn, prevError = console.error, prevGroup = console.group, prevGroupCollapsed = console.groupCollapsed, prevGroupEnd = console.groupEnd;
          var props = {
            configurable: true,
            enumerable: true,
            value: disabledLog,
            writable: true
          };
          Object.defineProperties(console, {
            info: props,
            log: props,
            warn: props,
            error: props,
            group: props,
            groupCollapsed: props,
            groupEnd: props
          });
        }
        disabledDepth++;
      }
    }
    function reenableLogs() {
      {
        if (disabledDepth--, disabledDepth === 0) {
          var props = {
            configurable: true,
            enumerable: true,
            writable: true
          };
          Object.defineProperties(console, {
            log: assign({}, props, {
              value: prevLog
            }),
            info: assign({}, props, {
              value: prevInfo
            }),
            warn: assign({}, props, {
              value: prevWarn
            }),
            error: assign({}, props, {
              value: prevError
            }),
            group: assign({}, props, {
              value: prevGroup
            }),
            groupCollapsed: assign({}, props, {
              value: prevGroupCollapsed
            }),
            groupEnd: assign({}, props, {
              value: prevGroupEnd
            })
          });
        }
        disabledDepth < 0 && error("disabledDepth fell below zero. This is a bug in React. Please file an issue.");
      }
    }
    var ReactCurrentDispatcher$1 = ReactSharedInternals.ReactCurrentDispatcher, prefix;
    function describeBuiltInComponentFrame(name, source, ownerFn) {
      {
        if (prefix === void 0)
          try {
            throw Error();
          } catch (x) {
            var match = x.stack.trim().match(/\n( *(at )?)/);
            prefix = match && match[1] || "";
          }
        return "\n" + prefix + name;
      }
    }
    var reentry = false, componentFrameCache;
    {
      var PossiblyWeakMap = typeof WeakMap == "function" ? WeakMap : Map;
      componentFrameCache = new PossiblyWeakMap();
    }
    function describeNativeComponentFrame(fn, construct) {
      if (!fn || reentry)
        return "";
      {
        var frame = componentFrameCache.get(fn);
        if (frame !== void 0)
          return frame;
      }
      var control;
      reentry = true;
      var previousPrepareStackTrace = Error.prepareStackTrace;
      Error.prepareStackTrace = void 0;
      var previousDispatcher;
      previousDispatcher = ReactCurrentDispatcher$1.current, ReactCurrentDispatcher$1.current = null, disableLogs();
      try {
        if (construct) {
          var Fake = function() {
            throw Error();
          };
          if (Object.defineProperty(Fake.prototype, "props", {
            set: function() {
              throw Error();
            }
          }), typeof Reflect == "object" && Reflect.construct) {
            try {
              Reflect.construct(Fake, []);
            } catch (x) {
              control = x;
            }
            Reflect.construct(fn, [], Fake);
          } else {
            try {
              Fake.call();
            } catch (x) {
              control = x;
            }
            fn.call(Fake.prototype);
          }
        } else {
          try {
            throw Error();
          } catch (x) {
            control = x;
          }
          fn();
        }
      } catch (sample) {
        if (sample && control && typeof sample.stack == "string") {
          for (var sampleLines = sample.stack.split("\n"), controlLines = control.stack.split("\n"), s = sampleLines.length - 1, c = controlLines.length - 1; s >= 1 && c >= 0 && sampleLines[s] !== controlLines[c]; )
            c--;
          for (; s >= 1 && c >= 0; s--, c--)
            if (sampleLines[s] !== controlLines[c]) {
              if (s !== 1 || c !== 1)
                do
                  if (s--, c--, c < 0 || sampleLines[s] !== controlLines[c]) {
                    var _frame = "\n" + sampleLines[s].replace(" at new ", " at ");
                    return fn.displayName && _frame.includes("<anonymous>") && (_frame = _frame.replace("<anonymous>", fn.displayName)), typeof fn == "function" && componentFrameCache.set(fn, _frame), _frame;
                  }
                while (s >= 1 && c >= 0);
              break;
            }
        }
      } finally {
        reentry = false, ReactCurrentDispatcher$1.current = previousDispatcher, reenableLogs(), Error.prepareStackTrace = previousPrepareStackTrace;
      }
      var name = fn ? fn.displayName || fn.name : "", syntheticFrame = name ? describeBuiltInComponentFrame(name) : "";
      return typeof fn == "function" && componentFrameCache.set(fn, syntheticFrame), syntheticFrame;
    }
    function describeFunctionComponentFrame(fn, source, ownerFn) {
      return describeNativeComponentFrame(fn, false);
    }
    function shouldConstruct(Component) {
      var prototype = Component.prototype;
      return !!(prototype && prototype.isReactComponent);
    }
    function describeUnknownElementTypeFrameInDEV(type, source, ownerFn) {
      if (type == null)
        return "";
      if (typeof type == "function")
        return describeNativeComponentFrame(type, shouldConstruct(type));
      if (typeof type == "string")
        return describeBuiltInComponentFrame(type);
      switch (type) {
        case REACT_SUSPENSE_TYPE:
          return describeBuiltInComponentFrame("Suspense");
        case REACT_SUSPENSE_LIST_TYPE:
          return describeBuiltInComponentFrame("SuspenseList");
      }
      if (typeof type == "object")
        switch (type.$$typeof) {
          case REACT_FORWARD_REF_TYPE:
            return describeFunctionComponentFrame(type.render);
          case REACT_MEMO_TYPE:
            return describeUnknownElementTypeFrameInDEV(type.type, source, ownerFn);
          case REACT_LAZY_TYPE: {
            var lazyComponent = type, payload = lazyComponent._payload, init = lazyComponent._init;
            try {
              return describeUnknownElementTypeFrameInDEV(init(payload), source, ownerFn);
            } catch (x) {
            }
          }
        }
      return "";
    }
    var loggedTypeFailures = {}, ReactDebugCurrentFrame$1 = ReactSharedInternals.ReactDebugCurrentFrame;
    function setCurrentlyValidatingElement(element) {
      if (element) {
        var owner = element._owner, stack = describeUnknownElementTypeFrameInDEV(element.type, element._source, owner ? owner.type : null);
        ReactDebugCurrentFrame$1.setExtraStackFrame(stack);
      } else
        ReactDebugCurrentFrame$1.setExtraStackFrame(null);
    }
    function checkPropTypes(typeSpecs, values, location, componentName, element) {
      {
        var has = Function.call.bind(hasOwnProperty);
        for (var typeSpecName in typeSpecs)
          if (has(typeSpecs, typeSpecName)) {
            var error$1 = void 0;
            try {
              if (typeof typeSpecs[typeSpecName] != "function") {
                var err = Error((componentName || "React class") + ": " + location + " type `" + typeSpecName + "` is invalid; it must be a function, usually from the `prop-types` package, but received `" + typeof typeSpecs[typeSpecName] + "`.This often happens because of typos such as `PropTypes.function` instead of `PropTypes.func`.");
                throw err.name = "Invariant Violation", err;
              }
              error$1 = typeSpecs[typeSpecName](values, typeSpecName, componentName, location, null, "SECRET_DO_NOT_PASS_THIS_OR_YOU_WILL_BE_FIRED");
            } catch (ex) {
              error$1 = ex;
            }
            error$1 && !(error$1 instanceof Error) && (setCurrentlyValidatingElement(element), error("%s: type specification of %s `%s` is invalid; the type checker function must return `null` or an `Error` but returned a %s. You may have forgotten to pass an argument to the type checker creator (arrayOf, instanceOf, objectOf, oneOf, oneOfType, and shape all require an argument).", componentName || "React class", location, typeSpecName, typeof error$1), setCurrentlyValidatingElement(null)), error$1 instanceof Error && !(error$1.message in loggedTypeFailures) && (loggedTypeFailures[error$1.message] = true, setCurrentlyValidatingElement(element), error("Failed %s type: %s", location, error$1.message), setCurrentlyValidatingElement(null));
          }
      }
    }
    function setCurrentlyValidatingElement$1(element) {
      if (element) {
        var owner = element._owner, stack = describeUnknownElementTypeFrameInDEV(element.type, element._source, owner ? owner.type : null);
        setExtraStackFrame(stack);
      } else
        setExtraStackFrame(null);
    }
    var propTypesMisspellWarningShown;
    propTypesMisspellWarningShown = false;
    function getDeclarationErrorAddendum() {
      if (ReactCurrentOwner.current) {
        var name = getComponentNameFromType(ReactCurrentOwner.current.type);
        if (name)
          return "\n\nCheck the render method of `" + name + "`.";
      }
      return "";
    }
    function getSourceInfoErrorAddendum(source) {
      if (source !== void 0) {
        var fileName = source.fileName.replace(/^.*[\\\/]/, ""), lineNumber = source.lineNumber;
        return "\n\nCheck your code at " + fileName + ":" + lineNumber + ".";
      }
      return "";
    }
    function getSourceInfoErrorAddendumForProps(elementProps) {
      return elementProps != null ? getSourceInfoErrorAddendum(elementProps.__source) : "";
    }
    var ownerHasKeyUseWarning = {};
    function getCurrentComponentErrorInfo(parentType) {
      var info = getDeclarationErrorAddendum();
      if (!info) {
        var parentName = typeof parentType == "string" ? parentType : parentType.displayName || parentType.name;
        parentName && (info = "\n\nCheck the top-level render call using <" + parentName + ">.");
      }
      return info;
    }
    function validateExplicitKey(element, parentType) {
      if (!(!element._store || element._store.validated || element.key != null)) {
        element._store.validated = true;
        var currentComponentErrorInfo = getCurrentComponentErrorInfo(parentType);
        if (!ownerHasKeyUseWarning[currentComponentErrorInfo]) {
          ownerHasKeyUseWarning[currentComponentErrorInfo] = true;
          var childOwner = "";
          element && element._owner && element._owner !== ReactCurrentOwner.current && (childOwner = " It was passed a child from " + getComponentNameFromType(element._owner.type) + "."), setCurrentlyValidatingElement$1(element), error('Each child in a list should have a unique "key" prop.%s%s See https://reactjs.org/link/warning-keys for more information.', currentComponentErrorInfo, childOwner), setCurrentlyValidatingElement$1(null);
        }
      }
    }
    function validateChildKeys(node, parentType) {
      if (typeof node == "object") {
        if (isArray(node))
          for (var i = 0; i < node.length; i++) {
            var child = node[i];
            isValidElement(child) && validateExplicitKey(child, parentType);
          }
        else if (isValidElement(node))
          node._store && (node._store.validated = true);
        else if (node) {
          var iteratorFn = getIteratorFn(node);
          if (typeof iteratorFn == "function" && iteratorFn !== node.entries)
            for (var iterator = iteratorFn.call(node), step; !(step = iterator.next()).done; )
              isValidElement(step.value) && validateExplicitKey(step.value, parentType);
        }
      }
    }
    function validatePropTypes(element) {
      {
        var type = element.type;
        if (type == null || typeof type == "string")
          return;
        var propTypes;
        if (typeof type == "function")
          propTypes = type.propTypes;
        else if (typeof type == "object" && (type.$$typeof === REACT_FORWARD_REF_TYPE || type.$$typeof === REACT_MEMO_TYPE))
          propTypes = type.propTypes;
        else
          return;
        if (propTypes) {
          var name = getComponentNameFromType(type);
          checkPropTypes(propTypes, element.props, "prop", name, element);
        } else if (type.PropTypes !== void 0 && !propTypesMisspellWarningShown) {
          propTypesMisspellWarningShown = true;
          var _name = getComponentNameFromType(type);
          error("Component %s declared `PropTypes` instead of `propTypes`. Did you misspell the property assignment?", _name || "Unknown");
        }
        typeof type.getDefaultProps == "function" && !type.getDefaultProps.isReactClassApproved && error("getDefaultProps is only used on classic React.createClass definitions. Use a static property named `defaultProps` instead.");
      }
    }
    function validateFragmentProps(fragment) {
      {
        for (var keys = Object.keys(fragment.props), i = 0; i < keys.length; i++) {
          var key = keys[i];
          if (key !== "children" && key !== "key") {
            setCurrentlyValidatingElement$1(fragment), error("Invalid prop `%s` supplied to `React.Fragment`. React.Fragment can only have `key` and `children` props.", key), setCurrentlyValidatingElement$1(null);
            break;
          }
        }
        fragment.ref !== null && (setCurrentlyValidatingElement$1(fragment), error("Invalid attribute `ref` supplied to `React.Fragment`."), setCurrentlyValidatingElement$1(null));
      }
    }
    function createElementWithValidation(type, props, children) {
      var validType = isValidElementType(type);
      if (!validType) {
        var info = "";
        (type === void 0 || typeof type == "object" && type !== null && Object.keys(type).length === 0) && (info += " You likely forgot to export your component from the file it's defined in, or you might have mixed up default and named imports.");
        var sourceInfo = getSourceInfoErrorAddendumForProps(props);
        sourceInfo ? info += sourceInfo : info += getDeclarationErrorAddendum();
        var typeString;
        type === null ? typeString = "null" : isArray(type) ? typeString = "array" : type !== void 0 && type.$$typeof === REACT_ELEMENT_TYPE ? (typeString = "<" + (getComponentNameFromType(type.type) || "Unknown") + " />", info = " Did you accidentally export a JSX literal instead of a component?") : typeString = typeof type, error("React.createElement: type is invalid -- expected a string (for built-in components) or a class/function (for composite components) but got: %s.%s", typeString, info);
      }
      var element = createElement.apply(this, arguments);
      if (element == null)
        return element;
      if (validType)
        for (var i = 2; i < arguments.length; i++)
          validateChildKeys(arguments[i], type);
      return type === REACT_FRAGMENT_TYPE ? validateFragmentProps(element) : validatePropTypes(element), element;
    }
    var didWarnAboutDeprecatedCreateFactory = false;
    function createFactoryWithValidation(type) {
      var validatedFactory = createElementWithValidation.bind(null, type);
      return validatedFactory.type = type, didWarnAboutDeprecatedCreateFactory || (didWarnAboutDeprecatedCreateFactory = true, warn("React.createFactory() is deprecated and will be removed in a future major release. Consider using JSX or use React.createElement() directly instead.")), Object.defineProperty(validatedFactory, "type", {
        enumerable: false,
        get: function() {
          return warn("Factory.type is deprecated. Access the class directly before passing it to createFactory."), Object.defineProperty(this, "type", {
            value: type
          }), type;
        }
      }), validatedFactory;
    }
    function cloneElementWithValidation(element, props, children) {
      for (var newElement = cloneElement.apply(this, arguments), i = 2; i < arguments.length; i++)
        validateChildKeys(arguments[i], newElement.type);
      return validatePropTypes(newElement), newElement;
    }
    function startTransition(scope, options) {
      var prevTransition = ReactCurrentBatchConfig.transition;
      ReactCurrentBatchConfig.transition = {};
      var currentTransition = ReactCurrentBatchConfig.transition;
      ReactCurrentBatchConfig.transition._updatedFibers = /* @__PURE__ */ new Set();
      try {
        scope();
      } finally {
        if (ReactCurrentBatchConfig.transition = prevTransition, prevTransition === null && currentTransition._updatedFibers) {
          var updatedFibersCount = currentTransition._updatedFibers.size;
          updatedFibersCount > 10 && warn("Detected a large number of updates inside startTransition. If this is due to a subscription please re-write it to use React provided hooks. Otherwise concurrent mode guarantees are off the table."), currentTransition._updatedFibers.clear();
        }
      }
    }
    var didWarnAboutMessageChannel = false, enqueueTaskImpl = null;
    function enqueueTask(task) {
      if (enqueueTaskImpl === null)
        try {
          var requireString = ("require" + Math.random()).slice(0, 7), nodeRequire = module && module[requireString];
          enqueueTaskImpl = nodeRequire.call(module, "timers").setImmediate;
        } catch (_err) {
          enqueueTaskImpl = function(callback) {
            didWarnAboutMessageChannel === false && (didWarnAboutMessageChannel = true, typeof MessageChannel == "undefined" && error("This browser does not have a MessageChannel implementation, so enqueuing tasks via await act(async () => ...) will fail. Please file an issue at https://github.com/facebook/react/issues if you encounter this warning."));
            var channel = new MessageChannel();
            channel.port1.onmessage = callback, channel.port2.postMessage(void 0);
          };
        }
      return enqueueTaskImpl(task);
    }
    var actScopeDepth = 0, didWarnNoAwaitAct = false;
    function act(callback) {
      {
        var prevActScopeDepth = actScopeDepth;
        actScopeDepth++, ReactCurrentActQueue.current === null && (ReactCurrentActQueue.current = []);
        var prevIsBatchingLegacy = ReactCurrentActQueue.isBatchingLegacy, result;
        try {
          if (ReactCurrentActQueue.isBatchingLegacy = true, result = callback(), !prevIsBatchingLegacy && ReactCurrentActQueue.didScheduleLegacyUpdate) {
            var queue = ReactCurrentActQueue.current;
            queue !== null && (ReactCurrentActQueue.didScheduleLegacyUpdate = false, flushActQueue(queue));
          }
        } catch (error) {
          throw popActScope(prevActScopeDepth), error;
        } finally {
          ReactCurrentActQueue.isBatchingLegacy = prevIsBatchingLegacy;
        }
        if (result !== null && typeof result == "object" && typeof result.then == "function") {
          var thenableResult = result, wasAwaited = false, thenable = {
            then: function(resolve, reject) {
              wasAwaited = true, thenableResult.then(function(returnValue) {
                popActScope(prevActScopeDepth), actScopeDepth === 0 ? recursivelyFlushAsyncActWork(returnValue, resolve, reject) : resolve(returnValue);
              }, function(error) {
                popActScope(prevActScopeDepth), reject(error);
              });
            }
          };
          return !didWarnNoAwaitAct && typeof Promise != "undefined" && Promise.resolve().then(function() {
          }).then(function() {
            wasAwaited || (didWarnNoAwaitAct = true, error("You called act(async () => ...) without await. This could lead to unexpected testing behaviour, interleaving multiple act calls and mixing their scopes. You should - await act(async () => ...);"));
          }), thenable;
        } else {
          var returnValue = result;
          if (popActScope(prevActScopeDepth), actScopeDepth === 0) {
            var _queue = ReactCurrentActQueue.current;
            _queue !== null && (flushActQueue(_queue), ReactCurrentActQueue.current = null);
            var _thenable = {
              then: function(resolve, reject) {
                ReactCurrentActQueue.current === null ? (ReactCurrentActQueue.current = [], recursivelyFlushAsyncActWork(returnValue, resolve, reject)) : resolve(returnValue);
              }
            };
            return _thenable;
          } else {
            var _thenable2 = {
              then: function(resolve, reject) {
                resolve(returnValue);
              }
            };
            return _thenable2;
          }
        }
      }
    }
    function popActScope(prevActScopeDepth) {
      prevActScopeDepth !== actScopeDepth - 1 && error("You seem to have overlapping act() calls, this is not supported. Be sure to await previous act() calls before making a new one. "), actScopeDepth = prevActScopeDepth;
    }
    function recursivelyFlushAsyncActWork(returnValue, resolve, reject) {
      {
        var queue = ReactCurrentActQueue.current;
        if (queue !== null)
          try {
            flushActQueue(queue), enqueueTask(function() {
              queue.length === 0 ? (ReactCurrentActQueue.current = null, resolve(returnValue)) : recursivelyFlushAsyncActWork(returnValue, resolve, reject);
            });
          } catch (error) {
            reject(error);
          }
        else
          resolve(returnValue);
      }
    }
    var isFlushing = false;
    function flushActQueue(queue) {
      if (!isFlushing) {
        isFlushing = true;
        var i = 0;
        try {
          for (; i < queue.length; i++) {
            var callback = queue[i];
            do
              callback = callback(true);
            while (callback !== null);
          }
          queue.length = 0;
        } catch (error) {
          throw queue = queue.slice(i + 1), error;
        } finally {
          isFlushing = false;
        }
      }
    }
    var createElement$1 = createElementWithValidation, cloneElement$1 = cloneElementWithValidation, createFactory = createFactoryWithValidation, Children = {
      map: mapChildren,
      forEach: forEachChildren,
      count: countChildren,
      toArray,
      only: onlyChild
    };
    exports.Children = Children, exports.Component = Component, exports.Fragment = REACT_FRAGMENT_TYPE, exports.Profiler = REACT_PROFILER_TYPE, exports.PureComponent = PureComponent, exports.StrictMode = REACT_STRICT_MODE_TYPE, exports.Suspense = REACT_SUSPENSE_TYPE, exports.__SECRET_INTERNALS_DO_NOT_USE_OR_YOU_WILL_BE_FIRED = ReactSharedInternals, exports.cloneElement = cloneElement$1, exports.createContext = createContext, exports.createElement = createElement$1, exports.createFactory = createFactory, exports.createRef = createRef, exports.forwardRef = forwardRef, exports.isValidElement = isValidElement, exports.lazy = lazy, exports.memo = memo, exports.startTransition = startTransition, exports.unstable_act = act, exports.useCallback = useCallback, exports.useContext = useContext, exports.useDebugValue = useDebugValue, exports.useDeferredValue = useDeferredValue, exports.useEffect = useEffect, exports.useId = useId, exports.useImperativeHandle = useImperativeHandle, exports.useInsertionEffect = useInsertionEffect, exports.useLayoutEffect = useLayoutEffect, exports.useMemo = useMemo, exports.useReducer = useReducer, exports.useRef = useRef, exports.useState = useState, exports.useSyncExternalStore = useSyncExternalStore, exports.useTransition = useTransition, exports.version = ReactVersion, typeof __REACT_DEVTOOLS_GLOBAL_HOOK__ != "undefined" && typeof __REACT_DEVTOOLS_GLOBAL_HOOK__.registerInternalModuleStop == "function" && __REACT_DEVTOOLS_GLOBAL_HOOK__.registerInternalModuleStop(new Error());
  })();
}) },[]]);
webpackJsonp.push([["0"], {"/Users/yanbo.wu/Documents/projects/tmp/esbuild/test/sourcemap/app/node_modules/react/index.js": (function(module, exports, require) {
  "use strict";
  module.exports = require("/Users/yanbo.wu/Documents/projects/tmp/esbuild/test/sourcemap/app/node_modules/react/cjs/react.development.js");
}) },[]]);
webpackJsonp.push([["0"], {"/Users/yanbo.wu/Documents/projects/tmp/esbuild/test/sourcemap/app/moduleA.tsx": (function(module, exports, require) {
  "use strict";
  function test() {
    throw new Error("test");
  }
  exports.test = test;
  exports.__esModule = true;
}) },[]]);
webpackJsonp.push([["0"], {"/Users/yanbo.wu/Documents/projects/tmp/esbuild/test/sourcemap/app/index.tsx": (function(module, exports, require) {
  "use strict";
  var React = (0, require.n)(require("/Users/yanbo.wu/Documents/projects/tmp/esbuild/test/sourcemap/app/node_modules/react/index.js"));
  var _ESPACK_MODULE_0 = require("/Users/yanbo.wu/Documents/projects/tmp/esbuild/test/sourcemap/app/node_modules/react/index.js");
  var _ESPACK_MODULE_1 = require("/Users/yanbo.wu/Documents/projects/tmp/esbuild/test/sourcemap/app/moduleA.tsx");
  var $ReactRefreshRuntime$ = require("<react_refresh_runtime>");
  // ESM Transformed
  // ESM Transformed
  function Component() {
    let [state, setState] = (0, _ESPACK_MODULE_0.useState)(0);
    return /* @__PURE__ */ React().createElement("div", null, /* @__PURE__ */ React().createElement("h1", null, "ddd"));
  }
  function sum(numberA, numberB) {
    return numberA + numberB;
  }
  var apple = 10, orange = 20, total = sum(apple, orange);
  console.log(total), console.log((0, _ESPACK_MODULE_1.test)());
  var _$s0 = (0, require.$Refresh$.signature)();
  _$s0(Component, "d41d8cd98f00b204e9800998ecf8427e", false, function() {
    return [];
  });
  (0, require.$Refresh$.register)(Component, "Component");
  exports.__esModule = true;
  const $ReactRefreshModuleId$ = "/Users/yanbo.wu/Documents/projects/tmp/esbuild/test/sourcemap/app/index.tsx";
  const $ReactRefreshCurrentExports$ = $ReactRefreshRuntime$.getModuleExports(
    $ReactRefreshModuleId$
  );
  function $ReactRefreshModuleRuntime$(exports) {
    return $ReactRefreshRuntime$.executeRuntime(
      exports,
      $ReactRefreshModuleId$,
      module.hot,
      false,
      $ReactRefreshModuleId$
    );
  }
  if (typeof Promise !== "undefined" && $ReactRefreshCurrentExports$ instanceof Promise) {
    $ReactRefreshCurrentExports$.then($ReactRefreshModuleRuntime$);
  } else {
    $ReactRefreshModuleRuntime$($ReactRefreshCurrentExports$);
  }
}) },[]]);
webpackJsonp.push([["0"], {"<initial_module_4>": (function(module, exports, require) {
  require("<dev_client>");
  require("<react_refresh_runtime>");
  require("/Users/yanbo.wu/Documents/projects/tmp/esbuild/test/sourcemap/app/index.tsx");
}) },[["<initial_module_4>"]]]);
//# sourceMappingURL=index.js.map