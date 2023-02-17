package server

import (
	"context"
	"encoding/json"
	"go-drive/common"
	"go-drive/common/drive_util"
	err "go-drive/common/errors"
	"go-drive/common/event"
	"go-drive/common/i18n"
	"go-drive/common/registry"
	"go-drive/common/types"
	"go-drive/common/utils"
	"go-drive/drive"
	"go-drive/drive/script"
	"go-drive/server/search"
	"go-drive/storage"
	"os"
	path2 "path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

func InitAdminRoutes(
	r gin.IRouter,
	ch *registry.ComponentsHolder,
	config common.Config,
	bus event.Bus,
	access *drive.Access,
	rootDrive *drive.RootDrive,
	search *search.Service,
	tokenStore types.TokenStore,
	optionsDAO *storage.OptionsDAO,
	userDAO *storage.UserDAO,
	groupDAO *storage.GroupDAO,
	driveDAO *storage.DriveDAO,
	driveDataDAO *storage.DriveDataDAO,
	permissionDAO *storage.PathPermissionDAO,
	pathMountDAO *storage.PathMountDAO,
	pathMetaDAO *storage.PathMetaDAO) error {

	r = r.Group("/admin", TokenAuth(tokenStore), AdminGroupRequired())

	// region user

	// list users
	r.GET("/users", func(c *gin.Context) {
		users, e := userDAO.ListUser()
		if e != nil {
			_ = c.Error(e)
			return
		}
		for i := range users {
			users[i].Password = ""
		}
		SetResult(c, users)
	})

	// get user by username
	r.GET("/user/:username", func(c *gin.Context) {
		username := c.Param("username")
		user, e := userDAO.GetUser(username)
		if e != nil {
			_ = c.Error(e)
			return
		}
		user.Password = ""
		SetResult(c, user)
	})

	// create user
	r.POST("/user", func(c *gin.Context) {
		user := types.User{}
		if e := c.Bind(&user); e != nil {
			_ = c.Error(e)
			return
		}
		addUser, e := userDAO.AddUser(user)
		if e != nil {
			_ = c.Error(e)
			return
		}
		SetResult(c, addUser)
	})

	// update user
	r.PUT("/user/:username", func(c *gin.Context) {
		user := types.User{}
		if e := c.Bind(&user); e != nil {
			_ = c.Error(e)
			return
		}
		username := c.Param("username")
		e := userDAO.UpdateUser(username, user)
		if e != nil {
			_ = c.Error(e)
			return
		}
	})

	// delete user
	r.DELETE("/user/:username", func(c *gin.Context) {
		username := c.Param("username")
		e := userDAO.DeleteUser(username)
		if e != nil {
			_ = c.Error(e)
			return
		}
	})

	// endregion

	// region group

	// list groups
	r.GET("/groups", func(c *gin.Context) {
		groups, e := groupDAO.ListGroup()
		if e != nil {
			_ = c.Error(e)
			return
		}
		SetResult(c, groups)
	})

	// get group and it's users
	r.GET("/group/:name", func(c *gin.Context) {
		name := c.Param("name")
		group, e := groupDAO.GetGroup(name)
		if e != nil {
			_ = c.Error(e)
			return
		}
		SetResult(c, group)
	})

	// create group
	r.POST("/group", func(c *gin.Context) {
		group := storage.GroupWithUsers{}
		if e := c.Bind(&group); e != nil {
			_ = c.Error(e)
			return
		}
		addGroup, e := groupDAO.AddGroup(group)
		if e != nil {
			_ = c.Error(e)
			return
		}
		SetResult(c, addGroup)
	})

	r.PUT("/group/:name", func(c *gin.Context) {
		name := c.Param("name")
		gus := storage.GroupWithUsers{}
		if e := c.Bind(&gus); e != nil {
			_ = c.Error(e)
			return
		}
		if e := groupDAO.UpdateGroup(name, gus); e != nil {
			_ = c.Error(e)
			return
		}
	})

	// delete group
	r.DELETE("/group/:name", func(c *gin.Context) {
		name := c.Param("name")
		e := groupDAO.DeleteGroup(name)
		if e != nil {
			_ = c.Error(e)
			return
		}
	})
	// endregion

	// region drive

	// get drive factories
	r.GET("/drive-factories", func(c *gin.Context) {
		ds := drive_util.GetRegisteredDrives(config)
		sort.Slice(ds, func(i, j int) bool { return ds[i].Type < ds[j].Type })
		SetResult(c, ds)
	})

	// get drives
	r.GET("/drives", func(c *gin.Context) {
		drives, e := driveDAO.GetDrives()
		if e != nil {
			_ = c.Error(e)
			return
		}
		for i, d := range drives {
			f := drive_util.GetDrive(d.Type, config)
			if f == nil {
				continue
			}
			drives[i].Config = escapeDriveConfigSecrets(f.ConfigForm, d.Config)
		}
		SetResult(c, drives)
	})

	// add drive
	r.POST("/drive", func(c *gin.Context) {
		d := types.Drive{}
		if e := c.Bind(&d); e != nil {
			_ = c.Error(e)
			return
		}
		if e := checkDriveName(d.Name); e != nil {
			_ = c.Error(e)
			return
		}
		d, e := driveDAO.AddDrive(d)
		if e != nil {
			_ = c.Error(e)
			return
		}
		SetResult(c, d)
	})

	// update drive
	r.PUT("/drive/:name", func(c *gin.Context) {
		name := c.Param("name")
		if e := checkDriveName(name); e != nil {
			_ = c.Error(e)
			return
		}
		d := types.Drive{}
		if e := c.Bind(&d); e != nil {
			_ = c.Error(e)
			return
		}
		f := drive_util.GetDrive(d.Type, config)
		if f == nil {
			_ = c.Error(err.NewNotAllowedMessageError(i18n.T("api.admin.unknown_drive_type", d.Type)))
			return
		}
		savedDrive, e := driveDAO.GetDrive(name)
		if e != nil {
			_ = c.Error(e)
			return
		}
		d.Config = unescapeDriveConfigSecrets(f.ConfigForm, savedDrive.Config, d.Config)
		e = driveDAO.UpdateDrive(name, d)
		if e != nil {
			_ = c.Error(e)
			return
		}
		_ = rootDrive.ClearDriveCache(name)
	})

	// delete drive
	r.DELETE("/drive/:name", func(c *gin.Context) {
		name := c.Param("name")
		e := driveDAO.DeleteDrive(name)
		_ = rootDrive.ClearDriveCache(name)
		_ = driveDataDAO.Remove(name)
		if e != nil {
			_ = c.Error(e)
			return
		}
	})

	// get drive initialization information
	r.POST("/drive/:name/init-config", func(c *gin.Context) {
		name := c.Param("name")
		data, e := rootDrive.DriveInitConfig(c.Request.Context(), name)
		if e != nil {
			_ = c.Error(e)
			return
		}
		SetResult(c, data)
	})

	// init drive
	r.POST("/drive/:name/init", func(c *gin.Context) {
		name := c.Param("name")
		data := make(types.SM, 0)
		if e := c.Bind(&data); e != nil {
			_ = c.Error(e)
			return
		}
		if e := rootDrive.DriveInit(c.Request.Context(), name, data); e != nil {
			_ = c.Error(e)
			return
		}
	})

	// reload drives
	r.POST("/drives/reload", func(c *gin.Context) {
		if e := rootDrive.ReloadDrive(c.Request.Context(), false); e != nil {
			_ = c.Error(e)
		}
	})

	// endregion

	// region permissions

	// get by path
	r.GET("/path-permissions/*path", func(c *gin.Context) {
		path := utils.CleanPath(c.Param("path"))
		permissions, e := permissionDAO.GetByPath(path)
		if e != nil {
			_ = c.Error(e)
			return
		}
		SetResult(c, permissions)
	})

	// save path permissions
	r.PUT("/path-permissions/*path", func(c *gin.Context) {
		path := utils.CleanPath(c.Param("path"))
		permissions := make([]types.PathPermission, 0)
		if e := c.Bind(&permissions); e != nil {
			_ = c.Error(e)
			return
		}
		if e := permissionDAO.SavePathPermissions(path, permissions); e != nil {
			_ = c.Error(e)
			return
		}
		// permissions updated
		if e := access.ReloadPerm(); e != nil {
			_ = c.Error(e)
			return
		}
	})

	// endregion

	// region path meta

	// get all path meta
	r.GET("/path-meta", func(c *gin.Context) {
		res, e := pathMetaDAO.GetAll()
		if e != nil {
			_ = c.Error(e)
			return
		}
		SetResult(c, res)
	})

	// create or add path meta
	r.POST("/path-meta/*path", func(c *gin.Context) {
		path := utils.CleanPath(c.Param("path"))
		data := types.PathMeta{}
		if e := c.Bind(&data); e != nil {
			_ = c.Error(e)
			return
		}
		data.Path = &path
		if e := pathMetaDAO.Set(data); e != nil {
			_ = c.Error(e)
			return
		}
	})

	// delete path meta by path
	r.DELETE("/path-meta/*path", func(c *gin.Context) {
		path := utils.CleanPath(c.Param("path"))
		if e := pathMetaDAO.Delete(path); e != nil {
			_ = c.Error(e)
			return
		}
	})

	// endregion

	// region mount

	// save mounts
	r.POST("/mount/*to", func(c *gin.Context) {
		s := GetSession(c)
		to := utils.CleanPath(c.Param("to"))
		src := make([]mountSource, 0)
		if e := c.Bind(&src); e != nil {
			_ = c.Error(e)
			return
		}
		if len(src) == 0 {
			return
		}

		dd := rootDrive.Get()

		var e error
		mounts := make([]types.PathMount, len(src))
		for i, p := range src {
			mountPath := utils.CleanPath(path2.Join(to, p.Name))
			mountPath, e = dd.FindNonExistsEntryName(c.Request.Context(), dd, mountPath)
			if e != nil {
				_ = c.Error(e)
				return
			}
			mounts[i] = types.PathMount{Path: &to, Name: utils.PathBase(mountPath), MountAt: p.Path}
		}
		if e := pathMountDAO.SaveMounts(mounts, true); e != nil {
			_ = c.Error(e)
			return
		}
		_ = rootDrive.ReloadMounts()
		for _, m := range mounts {
			bus.Publish(event.EntryUpdated, types.DriveListenerContext{
				Session: &s,
				Drive:   rootDrive.Get(),
			}, path2.Join(*m.Path, m.Name), true)
		}
	})

	// endregion

	// region search

	// index files
	r.POST("/search/index/*path", func(c *gin.Context) {
		root := utils.CleanPath(c.Param("path"))
		t, e := search.TriggerIndexAll(root, true)
		if e != nil {
			_ = c.Error(e)
			return
		}
		SetResult(c, t)
	})
	// endregion

	// region misc

	// clean all PathPermission and PathMount that is point to invalid path
	r.POST("/clean-permissions-mounts", func(c *gin.Context) {
		root := rootDrive.Get()
		pps, e := permissionDAO.GetAll()
		if e != nil {
			_ = c.Error(e)
			return
		}
		ms, e := pathMountDAO.GetMounts()
		if e != nil {
			_ = c.Error(e)
			return
		}
		paths := make(map[string]bool)
		var reloadPermission, reloadMount bool
		for _, p := range pps {
			paths[*p.Path] = true
			reloadPermission = true
		}
		for _, m := range ms {
			paths[m.MountAt] = true
			reloadMount = true
		}
		for p := range paths {
			_, e := root.Get(c.Request.Context(), p)
			if e != nil {
				if err.IsNotFoundError(e) {
					paths[p] = false
					continue
				}
				_ = c.Error(e)
				return
			}
		}
		n := 0
		for p, ok := range paths {
			if ok {
				continue
			}
			if e := permissionDAO.DeleteByPath(p); e != nil {
				_ = c.Error(e)
				return
			}
			if e := pathMountDAO.DeleteByMountAt(p); e != nil {
				_ = c.Error(e)
				return
			}
			n++
		}
		if reloadMount {
			_ = rootDrive.ReloadMounts()
		}
		if reloadPermission {
			_ = access.ReloadPerm()
		}
		SetResult(c, n)
	})

	// get service stats
	r.GET("/stats", func(c *gin.Context) {
		stats := ch.Gets(func(c interface{}) bool {
			_, ok := c.(types.IStatistics)
			return ok
		})
		res := make([]statItem, len(stats))
		for i, s := range stats {
			name, data, e := s.(types.IStatistics).Status()
			if e != nil {
				_ = c.Error(e)
				return
			}
			res[i] = statItem{Name: name, Data: data}
		}
		sort.Slice(res, func(i, j int) bool {
			return res[i].Name < res[j].Name
		})
		SetResult(c, res)
	})

	// clean drive cache
	r.DELETE("/drive-cache/:name", func(c *gin.Context) {
		name := c.Param("name")
		e := rootDrive.ClearDriveCache(name)
		if e != nil {
			_ = c.Error(e)
			return
		}
	})

	// set options
	r.PUT("/options", func(c *gin.Context) {
		options := make(map[string]string)
		if e := c.Bind(&options); e != nil {
			_ = c.Error(e)
			return
		}
		e := optionsDAO.Sets(options)
		if e != nil {
			_ = c.Error(e)
			return
		}
	})

	// get option
	r.GET("/options/:keys", func(c *gin.Context) {
		keys := strings.Split(c.Param("keys"), ",")
		value, e := optionsDAO.Gets(keys...)
		if e != nil {
			_ = c.Error(e)
			return
		}
		SetResult(c, value)
	})

	// endregion

	// region script drives

	scriptDriveRoutes := r.Group("/scripts")
	driveRepositoryLock := sync.Mutex{}

	var loadAvailableDriveScripts = func(ctx context.Context, forceLoad bool) ([]script.AvailableDriveScript, error) {
		driveRepositoryLock.Lock()
		defer driveRepositoryLock.Unlock()

		cacheFile := filepath.Join(config.TempDir, "drives-repository-cache.json")

		var result []script.AvailableDriveScript = nil

		if !forceLoad {
			if data, e := os.ReadFile(cacheFile); e == nil {
				temp := make([]script.AvailableDriveScript, 0)
				if e := json.Unmarshal(data, &temp); e == nil {
					result = temp
				}
			}
		}

		if result == nil {
			scripts, e := script.ListAvailableScriptsFromRepository(ctx, config.DriveRepositoryURL)
			if e != nil {
				return result, e
			}
			result = scripts

			data, e := json.Marshal(scripts)
			if e != nil {
				return result, e
			}
			if e := os.WriteFile(cacheFile, data, 0644); e != nil {
				return result, e
			}
		}
		return result, nil
	}

	// get available drives from repository
	scriptDriveRoutes.GET("/available", func(c *gin.Context) {
		result, e := loadAvailableDriveScripts(c.Request.Context(), utils.ToBool(c.Query("force")))
		if e != nil {
			_ = c.Error(e)
			return
		}
		SetResult(c, result)
	})

	// get installed drives
	scriptDriveRoutes.GET("/installed", func(c *gin.Context) {
		scripts, e := script.ListDriveScripts(config)
		if e != nil {
			_ = c.Error(e)
			return
		}
		SetResult(c, scripts)
	})

	scriptDriveRoutes.POST("/install/:name", func(c *gin.Context) {
		name := c.Param("name")
		scripts, e := loadAvailableDriveScripts(c.Request.Context(), false)
		if e != nil {
			_ = c.Error(e)
			return
		}

		ads, ok := utils.ArrayFind(scripts, func(item script.AvailableDriveScript, _ int) bool { return item.Name == name })
		if !ok {
			_ = c.Error(err.NewNotFoundError())
			return
		}

		if e := script.InstallDriveScript(c.Request.Context(), config, ads); e != nil {
			_ = c.Error(e)
			return
		}
	})

	scriptDriveRoutes.DELETE("/uninstall/:name", func(c *gin.Context) {
		name := c.Param("name")
		if e := script.UninstallDriveScript(config, name); e != nil {
			_ = c.Error(e)
			return
		}
	})

	scriptDriveRoutes.GET("/content/:name", func(c *gin.Context) {
		content, e := script.GetDriveScript(config, c.Param("name"))
		if e != nil {
			_ = c.Error(e)
			return
		}
		SetResult(c, content)
	})

	scriptDriveRoutes.PUT("/content/:name", func(c *gin.Context) {
		content := script.DriveScriptContent{}
		if e := c.Bind(&content); e != nil {
			_ = c.Error(e)
			return
		}
		if e := script.SaveDriveScript(config, c.Param("name"), content); e != nil {
			_ = c.Error(e)
			return
		}
	})

	// endregion

	return nil
}

type mountSource struct {
	Path string `json:"path" binding:"required"`
	Name string `json:"name" binding:"required"`
}

type statItem struct {
	Name string   `json:"name"`
	Data types.SM `json:"data"`
}

var driveNamePattern = regexp.MustCompile("^[^/\\\x00:*\"<>|]+$")

func checkDriveName(name string) error {
	if name == "" || name == "." || name == ".." || !driveNamePattern.MatchString(name) {
		return err.NewBadRequestError(i18n.T("api.admin.invalid_drive_name", name))
	}
	return nil
}

const escapedPassword = "YOU CAN'T SEE ME"

func escapeDriveConfigSecrets(form []types.FormItem, config string) string {
	val := types.SM{}
	_ = json.Unmarshal([]byte(config), &val)
	for _, f := range form {
		if (f.Type == "password" || f.Secret != "") && val[f.Field] != "" {
			val[f.Field] = escapedPassword
			if f.Secret != "" {
				val[f.Field] = f.Secret
			}
		}
	}
	s, _ := json.Marshal(val)
	return string(s)
}

func unescapeDriveConfigSecrets(form []types.FormItem, savedConfig string, config string) string {
	savedVal := types.SM{}
	val := types.SM{}
	_ = json.Unmarshal([]byte(savedConfig), &savedVal)
	_ = json.Unmarshal([]byte(config), &val)
	for _, f := range form {
		if (f.Type == "password" || f.Secret != "") &&
			(val[f.Field] == escapedPassword || (f.Secret != "" && val[f.Field] == f.Secret)) {
			val[f.Field] = savedVal[f.Field]
		}
	}
	s, _ := json.Marshal(val)
	return string(s)
}
