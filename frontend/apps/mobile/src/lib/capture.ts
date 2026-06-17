// Live-photo capture helper for clock-in (E5 F5.1, CI-10 2026-06-17). The mobile
// clock-in requires a live selfie: capture via the device camera, upload it, then
// pass the returned SWP-FILE-* id as `photo_id`. We use expo-image-picker's
// `launchCameraAsync` (expo-camera is intentionally NOT a dependency) so the system
// camera UI guarantees a freshly-taken photo rather than a library pick.
import * as ImagePicker from 'expo-image-picker';

/** Outcome of a capture attempt — discriminated so callers can branch on the reason. */
export type CaptureResult =
  | { status: 'ok'; file: Blob; fileName: string; mimeType: string }
  | { status: 'denied' } // camera permission refused
  | { status: 'cancelled' }; // user backed out of the camera UI

const DEFAULT_MIME = 'image/jpeg';

/**
 * Launch the system camera, request permission if needed, and return the captured
 * photo as a `Blob` (the shape `useUploadAttendancePhoto` needs —
 * `UploadAttendancePhotoBody.file: Blob`). Handles permission-denied and user-cancel
 * by returning a typed result instead of throwing, so the screen can re-prompt or
 * surface the right copy. Genuine I/O failures still throw.
 */
export async function captureClockInPhoto(): Promise<CaptureResult> {
  const perm = await ImagePicker.requestCameraPermissionsAsync();
  if (!perm.granted) return { status: 'denied' };

  const result = await ImagePicker.launchCameraAsync({
    mediaTypes: 'images',
    allowsEditing: false,
    quality: 0.6, // keep the upload well under the 10 MB cap
    cameraType: ImagePicker.CameraType.front, // selfie for clock-in verification
  });
  if (result.canceled) return { status: 'cancelled' };

  const asset = result.assets[0];
  if (!asset) return { status: 'cancelled' };

  // RN: fetch the local file:// URI and read it as a Blob for the multipart upload.
  const res = await fetch(asset.uri);
  const file = await res.blob();
  const mimeType = asset.mimeType ?? (file.type || DEFAULT_MIME);
  const fileName = asset.fileName ?? `clock-in-${Date.now()}.jpg`;
  return { status: 'ok', file, fileName, mimeType };
}
