import { useEffect, useState } from 'react';
import { useNavigate, useParams } from '@tanstack/react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, Camera, CheckCircle2, X } from 'lucide-react';
import { ApiError } from '@/api/client';
import {
  getLostFoundReport,
  updateLostFoundReport,
  type LostFoundType,
} from '@/api/lostfound';
import { uploadFile, listFiles } from '@/api/files';
import { fetchFileBlobURL } from '@/api/files';
import { WebcamPhotoCapture } from '@/components/WebcamPhotoCapture';
import { Button, LinkButton } from '@/components/ui/Button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card';
import { Dialog } from '@/components/ui/Dialog';
import { Field, Input, Textarea } from '@/components/ui/Input';
import { PageContainer, PageHeader } from '@/components/ui/PageHeader';
import { ErrorRow, LoadingRow } from '@/components/ui/States';
import { useMe, PERMS } from '@/hooks/useMe';
import { useSettings } from '@/hooks/useSettings';
import { isSacredLostFoundReport } from '@/lib/lostfoundSacred';
import { cn } from '@/lib/utils';

export function LostFoundEditPage() {
  const { id } = useParams({ strict: false }) as { id: string };
  const navigate = useNavigate();
  const qc = useQueryClient();
  const { can } = useMe();
  const settings = useSettings();

  const report = useQuery({
    queryKey: ['lostfound', id],
    queryFn: () => getLostFoundReport(id),
    retry: false,
  });

  const existingPhoto = useQuery({
    queryKey: ['lostfound-files', id],
    queryFn: () => listFiles({ entity_type: 'lostfound', entity_id: id, file_type: 'image' }),
    retry: false,
    enabled: !!report.data,
  });

  const [type, setType] = useState<LostFoundType | null>(null);
  const [itemName, setItemName] = useState('');
  const [description, setDescription] = useState('');
  const [location, setLocation] = useState('');
  const [contactName, setContactName] = useState('');
  const [contactInfo, setContactInfo] = useState('');
  const [photo, setPhoto] = useState<File | null>(null);
  const [photoPreview, setPhotoPreview] = useState<string | null>(null);
  const [removePhoto, setRemovePhoto] = useState(false);
  const [existingPhotoUrl, setExistingPhotoUrl] = useState<string | null>(null);
  const [typeConfirmOpen, setTypeConfirmOpen] = useState(false);
  const [pendingType, setPendingType] = useState<LostFoundType | null>(null);

  const initialType = report.data?.type;

  useEffect(() => {
    if (!report.data) return;
    const r = report.data;
    setType(r.type);
    setItemName(r.item_name);
    setDescription(r.description ?? '');
    setLocation(r.location ?? '');
    setContactName(r.contact_name ?? '');
    setContactInfo(r.contact_info ?? '');
  }, [report.data]);

  const fileId = existingPhoto.data?.items[0]?.id;
  useEffect(() => {
    let revoked: string | null = null;
    if (fileId && !photo && !removePhoto) {
      fetchFileBlobURL(fileId)
        .then((u) => {
          revoked = u;
          setExistingPhotoUrl(u);
        })
        .catch(() => setExistingPhotoUrl(null));
    } else {
      setExistingPhotoUrl(null);
    }
    return () => {
      if (revoked) URL.revokeObjectURL(revoked);
    };
  }, [fileId, photo, removePhoto]);

  const submit = useMutation({
    mutationFn: async () => {
      if (!type) throw new Error('Pick lost or found.');
      const body: Parameters<typeof updateLostFoundReport>[1] = {
        type,
        item_name: itemName.trim(),
        description: description.trim() || undefined,
        location: location.trim() || undefined,
        contact_name: contactName.trim() || undefined,
        contact_info: contactInfo.trim() || undefined,
      };

      if (removePhoto) {
        body.photo_file_id = '';
      } else if (photo) {
        const uploaded = await uploadFile(photo, {
          entity_type: 'lostfound',
          entity_id: id,
          file_type: 'image',
        });
        body.photo_file_id = uploaded.id;
      }

      return updateLostFoundReport(id, body);
    },
    onSuccess: (data) => {
      qc.setQueryData(['lostfound', id], data);
      qc.invalidateQueries({ queryKey: ['lostfound'] });
      qc.invalidateQueries({ queryKey: ['lostfound-files', id] });
      navigate({ to: '/lost-and-found/$id', params: { id } }).catch(() => undefined);
    },
  });

  function pickPhoto(file: File | null) {
    if (photoPreview) URL.revokeObjectURL(photoPreview);
    setPhoto(file);
    setPhotoPreview(file ? URL.createObjectURL(file) : null);
    if (file) setRemovePhoto(false);
  }

  function requestTypeChange(next: LostFoundType) {
    if (next === type) return;
    if (initialType && next !== initialType) {
      setPendingType(next);
      setTypeConfirmOpen(true);
      return;
    }
    setType(next);
  }

  const valid = !!type && itemName.trim().length > 0;

  if (!can(PERMS.lostfoundEdit)) {
    return (
      <PageContainer className="max-w-2xl">
        <ErrorRow message="You don't have permission to edit reports." />
        <div className="text-center">
          <LinkButton to="/lost-and-found/$id" params={{ id }} variant="ghost">
            Back to report
          </LinkButton>
        </div>
      </PageContainer>
    );
  }

  if (report.isLoading) {
    return (
      <PageContainer className="max-w-2xl">
        <LoadingRow />
      </PageContainer>
    );
  }

  if (report.isError || !report.data) {
    return (
      <PageContainer className="max-w-2xl">
        <ErrorRow message="Couldn't load this report." />
        <div className="text-center">
          <LinkButton to="/lost-and-found" variant="ghost">
            Back to Lost &amp; Found
          </LinkButton>
        </div>
      </PageContainer>
    );
  }

  if (report.data.status !== 'open') {
    return (
      <PageContainer className="max-w-2xl">
        <ErrorRow message="Only open reports can be edited. Reopen it first if needed." />
        <div className="text-center">
          <LinkButton to="/lost-and-found/$id" params={{ id }} variant="ghost">
            Back to report
          </LinkButton>
        </div>
      </PageContainer>
    );
  }

  if (
    isSacredLostFoundReport(
      report.data.report_number,
      !!settings.data?.preserve_sacred_lostfound
    )
  ) {
    return (
      <PageContainer className="max-w-2xl">
        <ErrorRow message="LF-0001 is the sacred test item and cannot be edited." />
        <div className="text-center">
          <LinkButton to="/lost-and-found/$id" params={{ id }} variant="ghost">
            Back to report
          </LinkButton>
        </div>
      </PageContainer>
    );
  }

  const previewSrc = photoPreview ?? (removePhoto ? null : existingPhotoUrl);

  return (
    <PageContainer className="max-w-2xl">
      <PageHeader
        title="Edit report"
        description={`${report.data.report_number} — fix mistakes or change lost ↔ found if the situation changed.`}
      />

      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (valid && !submit.isPending) submit.mutate();
        }}
        className="space-y-5"
      >
        <div className="grid grid-cols-2 gap-3">
          <TypeOption
            active={type === 'lost'}
            onClick={() => requestTypeChange('lost')}
            icon={AlertTriangle}
            tone="lost"
            title="Lost something"
            subtitle="Someone is missing an item"
          />
          <TypeOption
            active={type === 'found'}
            onClick={() => requestTypeChange('found')}
            icon={CheckCircle2}
            tone="found"
            title="Found something"
            subtitle="An item turned up"
          />
        </div>

        <Card>
          <CardHeader>
            <CardTitle>What is it?</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <Field label="Item" htmlFor="lf-edit-name">
              <Input
                id="lf-edit-name"
                value={itemName}
                onChange={(e) => setItemName(e.target.value)}
                required
              />
            </Field>
            <Field label="Description (optional)" htmlFor="lf-edit-desc">
              <Textarea
                id="lf-edit-desc"
                rows={3}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </Field>
            <Field
              label={type === 'found' ? 'Where was it found? (optional)' : 'Where was it last seen? (optional)'}
              htmlFor="lf-edit-loc"
            >
              <Input id="lf-edit-loc" value={location} onChange={(e) => setLocation(e.target.value)} />
            </Field>
            <Field label="Photo (optional)" hint="Upload a file, use the webcam, or remove the current photo.">
              {previewSrc ? (
                <div className="relative inline-block">
                  <img
                    src={previewSrc}
                    alt="Report"
                    className="max-h-44 rounded-lg border border-border object-cover"
                  />
                  <button
                    type="button"
                    onClick={() => {
                      pickPhoto(null);
                      setRemovePhoto(true);
                    }}
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
                    Upload a photo
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
            <CardTitle>Guest contact (optional)</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <Field label="Guest name" htmlFor="lf-edit-contact">
              <Input id="lf-edit-contact" value={contactName} onChange={(e) => setContactName(e.target.value)} />
            </Field>
            <Field label="Guest contact" htmlFor="lf-edit-contact-info">
              <Input id="lf-edit-contact-info" value={contactInfo} onChange={(e) => setContactInfo(e.target.value)} />
            </Field>
          </CardContent>
        </Card>

        {submit.isError ? (
          <p className="text-sm text-destructive" role="alert">
            {submit.error instanceof ApiError ? submit.error.message : 'Failed to save changes.'}
          </p>
        ) : null}

        <div className="flex gap-2">
          <Button type="submit" size="lg" disabled={!valid || submit.isPending}>
            {submit.isPending ? 'Saving…' : 'Save changes'}
          </Button>
          <LinkButton to="/lost-and-found/$id" params={{ id }} variant="ghost" size="lg">
            Cancel
          </LinkButton>
        </div>
      </form>

      <Dialog
        open={typeConfirmOpen}
        onClose={() => {
          setTypeConfirmOpen(false);
          setPendingType(null);
        }}
        title="Change report type?"
        description="Switching between lost and found changes how this report is listed and whether a label can be printed. Only do this if the situation actually changed."
      >
        <div className="flex gap-2 justify-end">
          <Button
            type="button"
            variant="ghost"
            onClick={() => {
              setTypeConfirmOpen(false);
              setPendingType(null);
            }}
          >
            Cancel
          </Button>
          <Button
            type="button"
            onClick={() => {
              if (pendingType) setType(pendingType);
              setTypeConfirmOpen(false);
              setPendingType(null);
            }}
          >
            Change type
          </Button>
        </div>
      </Dialog>
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
