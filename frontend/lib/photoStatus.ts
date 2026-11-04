// Image.status values; Unknown is client-only, for photos not yet polled.
export enum Status {
  Unknown = -1,
  Ready = 0,
  Processing = 1,
  Failed = 2,
}
