import { useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, Camera, CheckCircle2, X } from 'lucide-react';
import { ApiError } from '@/api/client';
import { createLostFoundReport, type LostFoundType } from '@/api/lostfound';
import { uploadFile } from '@/api/files';
import { WebcamPhotoCapture } from '@/components/WebcamPhotoCapture';
import { Button } from '@/components/ui/Button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/Card';
import { Field, Input, Textarea } from '@/components/ui/Input';
import { PageContainer, PageHeader } from '@/components/ui/PageHeader';
import { cn } from '@/lib/utils';

export function LostFoundNewPage() {
  const navigate = useNavigate();
  const qc = useQueryClient();

  const [type, setType] = useState<LostFoundType | null>(null);
  const [itemName, setItemName] = useState('');
  const [description, setDescription] = useState('');
  const [location, setLocation] = useState('');
  const [contactName, setContactName] = useState('');
  const [contactInfo, setContactInfo] = useState('');
  const [photo, setPhoto] = useState<File | null>(null);
  const [photoPreview, setPhotoPreview] = useState<string | null>(null);

  const submit = useMutation({
    mutationFn: async () => {
      if (!type) throw new Error('Pick lost or found first.');
      const report = await createLostFoundReport({
        type,
        item_name: itemName.trim(),
        description: description.trim() || undefined,
        location: location.trim() || undefined,
        contact_name: contactName.trim() || undefined,
        contact_info: contactInfo.trim() || undefined,
      });
      if (photo) {
        // Photo upload failing should not lose the report — it can be
        // re-added from the detail page later.
        await uploadFile(photo, {
          entity_type: 'lostfound',
          entity_id: report.id,
          file_type: 'image',
        }).catch(() => undefined);
      }
      return report;
    },
    onSuccess: (report) => {
      qc.invalidateQueries({ queryKey: ['lostfound'] });
      navigate({
        to: '/lost-and-found/$id',
        params: { id: report.id },
        search: { fresh: true },
      }).catch(() => undefined);
    },
  });

  function pickPhoto(file: File | null) {
    if (photoPreview) URL.revokeObjectURL(photoPreview);
    setPhoto(file);
    setPhotoPreview(file ? URL.createObjectURL(file) : null);
  }

  const valid = !!type && itemName.trim().length > 0;

  return (
    <PageContainer className="max-w-2xl">
      <PageHeader
        title="New report"
        description="Log something that went missing or something that turned up. We'll give it a number and keep track."
      />

      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (valid && !submit.isPending) submit.mutate();
        }}
        className="space-y-5"
      >
        {/* Type picker — the first, biggest decision */}
        <div className="grid grid-cols-2 gap-3">
          <TypeOption
            active={type === 'lost'}
            onClick={() => setType('lost')}
            icon={AlertTriangle}
            tone="lost"
            title="Lost something"
            subtitle="Someone is missing an item"
          />
          <TypeOption
            active={type === 'found'}
            onClick={() => setType('found')}
            icon={CheckCircle2}
            tone="found"
            title="Found something"
            subtitle="An item turned up"
          />
        </div>

        <Card>
          <CardHeader>
            <CardTitle>What is it?</CardTitle>
            <CardDescription>A clear name and description make matching much easier.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <Field label="Item" htmlFor="lf-name">
              <Input
                id="lf-name"
                value={itemName}
                onChange={(e) => setItemName(e.target.value)}
                placeholder="e.g. Blue backpack, Toyota keys with red keychain…"
                required
              />
            </Field>
            <Field label="Description (optional)" htmlFor="lf-desc">
              <Textarea
                id="lf-desc"
                rows={3}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Brand, color, stickers, contents — anything that identifies it."
              />
            </Field>
            <Field
              label={type === 'found' ? 'Where was it found? (optional)' : 'Where was it last seen? (optional)'}
              htmlFor="lf-loc"
            >
              <Input
                id="lf-loc"
                value={location}
                onChange={(e) => setLocation(e.target.value)}
                placeholder="e.g. left near the gym, main hall row C…"
              />
            </Field>
            <Field label="Photo (optional)" hint="Upload a file, use the webcam, or on a phone tap to take a picture.">
              {photoPreview ? (
                <div className="relative inline-block">
                  <img
                    src={photoPreview}
                    alt="Selected"
                    className="max-h-44 rounded-lg border border-border object-cover"
                  />
                  <button
                    type="button"
                    onClick={() => pickPhoto(null)}
                    className="absolute -right-2 -top-2 rounded-pill bg-destructive text-destructive-foreground p-1"
                    aria-label="Remove photo"
                  >
                    <X className="w-3.5 h-3.5" />
                  </button>
                </div>
              ) : (
                <div className="space-y-3">
                  <label className="flex items-center gap-2 rounded-md border border-dashed border-border px-4 py-6 cursor-pointer text-sm text-muted-foreground hover:bg-secondary transition-colors justify-center">
                    <Camera className="w-4 h-4" />
                    Upload or take a photo
                    <input
                      type="file"
                      accept="image/*"
                      className="sr-only"
                      onChange={(e) => pickPhoto(e.target.files?.[0] ?? null)}
                    />
                  </label>
                  <WebcamPhotoCapture onCapture={(file) => pickPhoto(file)} />
                </div>
              )}
            </Field>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Who is it about?</CardTitle>
            <CardDescription>
              You're recorded as the reporter automatically. If you're logging this for a guest, add their
              contact so we can reach them when it's matched.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            <Field label="Guest name (optional)" htmlFor="lf-contact">
              <Input
                id="lf-contact"
                value={contactName}
                onChange={(e) => setContactName(e.target.value)}
                placeholder="e.g. John Doe"
              />
            </Field>
            <Field label="Guest contact (optional)" htmlFor="lf-contact-info" hint="Telegram, phone, badge number — whatever works.">
              <Input
                id="lf-contact-info"
                value={contactInfo}
                onChange={(e) => setContactInfo(e.target.value)}
                placeholder="e.g. @johndoe / room 214"
              />
            </Field>
          </CardContent>
        </Card>

        {submit.isError ? (
          <p className="text-sm text-destructive" role="alert">
            {submit.error instanceof ApiError ? submit.error.message : 'Failed to create the report.'}
          </p>
        ) : null}

        <div className="flex gap-2">
          <Button type="submit" size="lg" disabled={!valid || submit.isPending}>
            {submit.isPending ? 'Saving…' : 'Create report'}
          </Button>
          <Button
            type="button"
            size="lg"
            variant="ghost"
            onClick={() => navigate({ to: '/lost-and-found' }).catch(() => undefined)}
          >
            Cancel
          </Button>
        </div>
      </form>
    </PageContainer>
  );
}

function TypeOption({
  active,
  onClick,
  icon: Icon,
  tone,
  title,
  subtitle,
}: {
  active: boolean;
  onClick: () => void;
  icon: React.ComponentType<{ className?: string }>;
  tone: 'lost' | 'found';
  title: string;
  subtitle: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={cn(
        'rounded-card border-2 p-4 sm:p-5 text-left transition-all',
        active
          ? tone === 'lost'
            ? 'border-destructive bg-destructive/10'
            : 'border-success bg-success/10'
          : 'border-border bg-card hover:border-muted-foreground/40'
      )}
    >
      <span
        className={cn(
          'inline-flex rounded-pill p-2 mb-2',
          tone === 'lost' ? 'bg-destructive/15 text-destructive' : 'bg-success/15 text-success'
        )}
      >
        <Icon className="w-5 h-5" />
      </span>
      <span className="block font-heading font-bold">{title}</span>
      <span className="block text-sm text-muted-foreground mt-0.5">{subtitle}</span>
    </button>
  );
}
