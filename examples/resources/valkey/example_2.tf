resource "bahriya_valkey" "queue" {
  handle        = "job-queue"
  name          = "Job Queue"
  purpose       = "store"
  tier          = "ha"
  size          = "standard"
  memorymb      = 1024
  storagegb     = 10
  activeregions = ["falkenstein-1"]
  project       = bahriya_project.web.id

  backupenabled  = true
  backupschedule = "0 3 * * *"
}
