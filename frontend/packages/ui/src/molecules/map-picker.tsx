/**
 * MapPicker — interactive Leaflet/OpenStreetMap geofence picker (E2 F2.6).
 * Click the map to set a site's geofence center; the radius circle previews the
 * `geofence_radius_m`. Controlled: the parent owns `value` (center) and `radiusM`.
 *
 * Leaflet path colors are SVG attributes set in JS, so design tokens (CSS vars) can't be
 * used for the circle — we mirror the brand primary token value (#188E4D, DESIGN-SYSTEM §2).
 * No marker icon is used (avoids the bundler default-icon issue); the center is a CircleMarker.
 */
import 'leaflet/dist/leaflet.css';
import { useEffect } from 'react';
import { Circle, CircleMarker, MapContainer, TileLayer, useMap, useMapEvents } from 'react-leaflet';
import { cn } from '../lib/cn.ts';

export interface LatLng {
  lat: number;
  lng: number;
}

export interface MapPickerProps {
  /** Geofence center, or null when not yet set. */
  value: LatLng | null;
  /** Radius in meters (previewed as a circle around the center). */
  radiusM: number;
  /** Fired when the user clicks a new center. */
  onChange: (next: LatLng) => void;
  /** Fallback view center when `value` is null. Default: Jakarta. */
  defaultCenter?: LatLng;
  /** Map height in px. Default 320. */
  height?: number;
  /** Read-only display (no click-to-set, no scroll-zoom). */
  readOnly?: boolean;
  className?: string;
}

const JAKARTA: LatLng = { lat: -6.2088, lng: 106.8456 };
const PRIMARY = '#188E4D';

function ClickCapture({
  onChange,
  readOnly,
}: { onChange: (n: LatLng) => void; readOnly?: boolean }) {
  useMapEvents({
    click(e) {
      if (!readOnly) onChange({ lat: e.latlng.lat, lng: e.latlng.lng });
    },
  });
  return null;
}

/** Pan to the center when it changes externally (e.g. lat/lng typed into the form fields). */
function Recenter({ center }: { center: LatLng }) {
  const map = useMap();
  useEffect(() => {
    map.setView([center.lat, center.lng], map.getZoom());
  }, [center.lat, center.lng, map]);
  return null;
}

export function MapPicker({
  value,
  radiusM,
  onChange,
  defaultCenter = JAKARTA,
  height = 320,
  readOnly,
  className,
}: MapPickerProps) {
  const center = value ?? defaultCenter;
  return (
    <div
      className={cn('overflow-hidden rounded-md border border-border', className)}
      style={{ height }}
    >
      <MapContainer
        center={[center.lat, center.lng]}
        zoom={value ? 16 : 12}
        scrollWheelZoom={!readOnly}
        className="size-full"
      >
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />
        <ClickCapture onChange={onChange} readOnly={readOnly} />
        {value && (
          <>
            <Recenter center={value} />
            <Circle
              center={[value.lat, value.lng]}
              radius={radiusM}
              pathOptions={{ color: PRIMARY, fillColor: PRIMARY, fillOpacity: 0.12, weight: 2 }}
            />
            <CircleMarker
              center={[value.lat, value.lng]}
              radius={6}
              pathOptions={{ color: PRIMARY, fillColor: PRIMARY, fillOpacity: 1, weight: 2 }}
            />
          </>
        )}
      </MapContainer>
    </div>
  );
}
