/// local storage provider

import http from '@/api/http'
import { taskDone } from '@/utils'
import { ApiError, RequestTask } from '@/utils/http/utils'
import UploadTask, {
  STATUS_COMPLETED,
  STATUS_ERROR,
  STATUS_STOPPED,
  STATUS_UPLOADING,
} from '../task'

/**
 * local upload task provider
 */
export default class LocalUploadTask extends UploadTask {
  private _httpTask?: RequestTask

  override async start() {
    if ((await super.start()) === false) return false

    let task = http.put(`/content/${this.task.path}`, this.task.file, {
      params: { override: this.task.override ? '1' : '' },
      transformRequest: (d) => d,
      onUploadProgress: ({ loaded, total }) => {
        this._onChange(STATUS_UPLOADING, { loaded, total })
      },
    })
    this._httpTask = task

    return taskDone(task)
      .then(
        () => {
          this._onChange(STATUS_COMPLETED)
        },
        (e) => {
          if (this.status === STATUS_STOPPED) return
          this._onChange(STATUS_ERROR, ApiError.from(e))
        }
      )
      .then(() => {
        this._httpTask = undefined
      })
  }

  override async pause() {
    console.warn('[LocalUploadTask] cannot pause task')
  }

  override async stop() {
    if (this._httpTask) {
      this._onChange(STATUS_STOPPED)
      this._httpTask.cancel()
    }
  }
}
