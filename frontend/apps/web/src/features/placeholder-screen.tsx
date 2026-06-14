import { StateView } from '@swp/ui';

/** Stand-in for not-yet-built feature screens, so nav targets render a designed state (B2). */
export function PlaceholderScreen({ title }: { title: string }) {
  return (
    <div className="space-y-6">
      <h1 className="font-bold text-3xl text-text">{title}</h1>
      <StateView
        kind="empty"
        title="Belum tersedia"
        description="Layar ini akan dibangun pada epik terkait."
      />
    </div>
  );
}
