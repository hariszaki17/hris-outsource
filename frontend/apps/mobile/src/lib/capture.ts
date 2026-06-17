// Live-photo capture helper for clock-in (E5 F5.1, CI-10 2026-06-17). The mobile
// clock-in requires a live selfie: capture via the device camera, downscale +
// recompress on-device, upload it, then pass the returned SWP-FILE-* id as
// `photo_id`. We use expo-image-picker's `launchCameraAsync` (expo-camera is
// intentionally NOT a dependency) so the system camera UI guarantees a freshly-taken
// photo rather than a library pick.
//
// PERF NOTE — agent phones are low-end. A clock-in selfie is presence proof, not a
// portfolio shot, so we shrink aggressively to keep upload + processing cheap:
//   - capture at low quality (small JPEG → cheap to decode on the manipulator pass);
//   - run exactly ONE manipulate pass, and ONLY when the source is wider than the
//     target (a phone whose camera already outputs <= MAX_WIDTH skips it entirely);
//   - manipulation failure (OOM on a weak device) falls back to the raw capture so a
//     clock-in is NEVER blocked by image processing.
import * as ImageManipulator from 'expo-image-manipulator';
import * as ImagePicker from 'expo-image-picker';

/** Outcome of a capture attempt — discriminated so callers can branch on the reason. */
export type CaptureResult =
  | { status: 'ok'; file: Blob; fileName: string; mimeType: string }
  | { status: 'denied' } // camera permission refused
  | { status: 'cancelled' }; // user backed out of the camera UI

const DEFAULT_MIME = 'image/jpeg';
const MAX_WIDTH = 1080; // longest-edge cap — ample for a verification selfie
const COMPRESS = 0.6; // 0..1; legible face at a fraction of the original bytes
const CAPTURE_QUALITY = 0.5; // small intermediate → cheaper decode on the resize pass

/**
 * Resize-if-needed + recompress, returning the processed file URI. Skips the
 * decode/encode pass entirely when `width` is already within `MAX_WIDTH` (the common
 * low-end case), and falls back to the original URI if manipulation throws.
 */
async function shrink(uri: string, width: number | undefined): Promise<string> {
  if (width !== undefined && width <= MAX_WIDTH) return uri; // already small — no pass
  try {
    const out = await ImageManipulator.manipulateAsync(
      uri,
      [{ resize: { width: MAX_WIDTH } }], // width-only → preserves aspect, one op
      { compress: COMPRESS, format: ImageManipulator.SaveFormat.JPEG },
    );
    return out.uri;
  } catch {
    return uri; // never block clock-in on a processing failure
  }
}

/**
 * Launch the system camera, request permission if needed, downscale on-device, and
 * return the captured photo as a `Blob` (the shape `useUploadAttendancePhoto` needs —
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
    quality: CAPTURE_QUALITY,
    cameraType: ImagePicker.CameraType.front, // selfie for clock-in verification
  });
  if (result.canceled) return { status: 'cancelled' };

  const asset = result.assets[0];
  if (!asset) return { status: 'cancelled' };

  // Downscale before reading bytes so the Blob (and the upload) is already small.
  const uri = await shrink(asset.uri, asset.width);

  // RN: fetch the local file:// URI and read it as a Blob for the multipart upload.
  const res = await fetch(uri);
  const file = await res.blob();
  // After our manipulate pass the output is always JPEG; trust the asset hint only
  // when we skipped processing.
  const mimeType = uri === asset.uri ? (asset.mimeType ?? file.type ?? DEFAULT_MIME) : DEFAULT_MIME;
  const fileName = asset.fileName ?? `clock-in-${Date.now()}.jpg`;
  return { status: 'ok', file, fileName, mimeType };
}
