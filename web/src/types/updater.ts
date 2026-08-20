export type UpdateCheckResult = {
  CurrentVersion: string
  LatestVersion: string
  HasUpdate: boolean
  ReleaseNotes: string
  DownloadURL: string
  Mandatory: boolean
  PublishedAt: string
  CheckError?: string
}
