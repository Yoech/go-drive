var global = this;

function __requireFunction__(name, e) {
  if (typeof e !== "function") {
    throw new Error(name + " is required to be implemented");
  }
}

function defineCreate(fn) {
  global.__driveCreate = function (ctx, config, utils) {
    var drive = fn(ctx, config, utils);
    if (!drive || typeof drive !== "object") {
      throw new Error("drive is not implemented");
    }
    var requiredFns = ["meta", "get", "list", "getReader"];
    for (var i = 0; i < requiredFns.length; i++) {
      __requireFunction__(requiredFns[i], drive[requiredFns[i]]);
    }
    var allFns = requiredFns.concat(
      "save",
      "makeDir",
      "copy",
      "move",
      "delete",
      "upload",
      "getURL",
      "getThumbnail",
      "hasThumbnail"
    );
    for (var i = 0; i < allFns.length; i++) {
      var fnName = allFns[i];
      if (typeof drive[fnName] !== "function") continue;
      global["__drive_" + fnName] = drive[fnName].bind(drive);
    }

    var props = Object.keys(drive).filter(function (key) {
      return key[0] === "$";
    });
    /**
     * @type {PropertyDescriptorMap}
     */
    var descriptors = {};
    var values = {};
    props.forEach(function (key) {
      descriptors[key] = {
        configurable: false,
        get: function () {
          return getData(key);
        },
        set: function (v) {
          var dat = {};
          dat[key] = v;
          setData(dat);
        },
        enumerable: true,
      };
      values[key] = drive[key];
    });

    setData(values);
    Object.defineProperties(drive, descriptors);
    Object.freeze(drive);
  };
}

function defineInitConfig(fn) {
  global.__driveInitConfig = function (ctx, config, utils) {
    return fn(ctx, config, utils);
  };
}

function defineInit(fn) {
  global.__driveInit = function (ctx, data, config, utils) {
    return fn(ctx, data, config, utils);
  };
}

var LocalProviderChunkSize = 1 * 1024 * 1024;

function useLocalProvider(size) {
  if (size <= LocalProviderChunkSize) {
    return { Provider: "local" };
  }
  return { Provider: "localChunk" };
}

function useCustomProvider(uploader, data) {
  return {
    Provider: "custom",
    Config: Object.assign({}, data, {
      uploader: uploader,
    }),
  };
}
