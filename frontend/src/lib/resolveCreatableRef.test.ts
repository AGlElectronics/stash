import { describe, expect, it, vi } from 'vitest';
import {
  commitCreatablePick,
  creatablePickFromDom,
  itemFormRefFields,
  normalizeCreatablePick,
  resolveCreatableRef,
} from './resolveCreatableRef';

describe('normalizeCreatablePick', () => {
  it('trims whitespace from name', () => {
    expect(normalizeCreatablePick({ name: '  Foo  ' })).toEqual({ name: 'Foo' });
  });

  it('returns id when name is empty but id exists', () => {
    expect(normalizeCreatablePick({ id: 'abc', name: '  ' })).toEqual({ id: 'abc', name: '' });
  });

  it('returns empty name when both are empty', () => {
    expect(normalizeCreatablePick({ name: '' })).toEqual({ name: '' });
  });

  it('preserves id when name is present', () => {
    expect(normalizeCreatablePick({ id: 'x', name: 'Bar' })).toEqual({ id: 'x', name: 'Bar' });
  });
});

describe('commitCreatablePick', () => {
  const options = [
    { id: '1', name: 'Alpha' },
    { id: '2', name: 'Beta' },
  ];

  it('matches existing option case-insensitively', () => {
    expect(commitCreatablePick({ name: 'alpha' }, options)).toEqual({ id: '1', name: 'Alpha' });
  });

  it('returns name-only for new values', () => {
    expect(commitCreatablePick({ name: 'Gamma' }, options)).toEqual({ name: 'Gamma' });
  });

  it('returns id+name when id matches and name matches that record', () => {
    expect(commitCreatablePick({ id: '1', name: 'Alpha' }, options)).toEqual({ id: '1', name: 'Alpha' });
  });

  it('falls through to name match when id does not match label', () => {
    expect(commitCreatablePick({ id: '1', name: 'Beta' }, options)).toEqual({ id: '2', name: 'Beta' });
  });

  it('handles empty name with id', () => {
    expect(commitCreatablePick({ id: '1', name: '' }, options)).toEqual({ id: '1', name: '' });
  });
});

describe('resolveCreatableRef', () => {
  it('returns undefined for empty name without id', async () => {
    const result = await resolveCreatableRef(
      { name: '' },
      vi.fn(),
      vi.fn(),
    );
    expect(result).toBeUndefined();
  });

  it('returns id directly when name is empty but id is set', async () => {
    const result = await resolveCreatableRef(
      { id: 'existing-id', name: '' },
      vi.fn(),
      vi.fn(),
    );
    expect(result).toBe('existing-id');
  });

  it('returns cached value without fetching', async () => {
    const cache = new Map([['hello', 'cached-id']]);
    const fetchList = vi.fn();
    const result = await resolveCreatableRef(
      { name: 'Hello' },
      fetchList,
      vi.fn(),
      cache,
    );
    expect(result).toBe('cached-id');
    expect(fetchList).not.toHaveBeenCalled();
  });

  it('finds existing item by name in fetched list', async () => {
    const fetchList = vi.fn().mockResolvedValue([
      { id: 'dept-1', name: 'Engineering' },
      { id: 'dept-2', name: 'Operations' },
    ]);
    const create = vi.fn();
    const result = await resolveCreatableRef(
      { name: 'Engineering' },
      fetchList,
      create,
    );
    expect(result).toBe('dept-1');
    expect(create).not.toHaveBeenCalled();
  });

  it('creates new item when not found in list', async () => {
    const fetchList = vi.fn().mockResolvedValue([]);
    const create = vi.fn().mockResolvedValue({ id: 'new-id', name: 'NewDept' });
    const result = await resolveCreatableRef(
      { name: 'NewDept' },
      fetchList,
      create,
    );
    expect(result).toBe('new-id');
    expect(create).toHaveBeenCalledWith('NewDept');
  });

  it('populates cache after successful create', async () => {
    const cache = new Map<string, string>();
    const fetchList = vi.fn().mockResolvedValue([]);
    const create = vi.fn().mockResolvedValue({ id: 'created-id', name: 'Test' });
    await resolveCreatableRef({ name: 'Test' }, fetchList, create, cache);
    expect(cache.get('test')).toBe('created-id');
  });

  it('retries on 409 and finds the created record', async () => {
    const { ApiError } = await import('@/api/client');
    let fetchCount = 0;
    const fetchList = vi.fn().mockImplementation(async () => {
      fetchCount++;
      if (fetchCount === 1) return [];
      return [{ id: 'race-id', name: 'RaceDept' }];
    });
    const err409 = new ApiError(409, { error: { code: 'conflict', message: 'already exists' } });
    const create = vi.fn().mockRejectedValue(err409);
    const result = await resolveCreatableRef(
      { name: 'RaceDept' },
      fetchList,
      create,
    );
    expect(result).toBe('race-id');
  });
});

describe('itemFormRefFields', () => {
  it('maps trimmed names for server-side resolve', () => {
    const fields = itemFormRefFields({
      category: { name: '  Cat  ' },
      managed_by_department: { name: 'test1' },
      needed_by_department: { name: 'test2' },
      location: { name: 'test3' },
    });
    expect(fields).toEqual({
      category_id: undefined,
      category_name: 'Cat',
      managed_by_department_id: undefined,
      managed_by_department_name: 'test1',
      needed_by_department_id: undefined,
      needed_by_department_name: 'test2',
      location_id: undefined,
      location_name: 'test3',
    });
  });

  it('includes ids when present', () => {
    const fields = itemFormRefFields({
      category: { id: 'cat-1', name: 'Electronics' },
      managed_by_department: { id: 'dept-1', name: 'IT' },
      needed_by_department: { name: '' },
      location: { id: 'loc-1', name: 'Room 5' },
    });
    expect(fields).toEqual({
      category_id: 'cat-1',
      category_name: 'Electronics',
      managed_by_department_id: 'dept-1',
      managed_by_department_name: 'IT',
      needed_by_department_id: undefined,
      needed_by_department_name: undefined,
      location_id: 'loc-1',
      location_name: 'Room 5',
    });
  });
});

describe('creatablePickFromDom', () => {
  it('prefers live DOM input text over stale form state', () => {
    document.body.innerHTML = '<input id="managed_by_department" value="from-dom" />';
    const pick = creatablePickFromDom('managed_by_department', { name: '' }, []);
    expect(pick).toEqual({ name: 'from-dom' });
  });

  it('falls back to pick name when DOM element missing', () => {
    document.body.innerHTML = '';
    const pick = creatablePickFromDom('nonexistent', { name: 'fallback' }, []);
    expect(pick).toEqual({ name: 'fallback' });
  });

  it('matches DOM text against options list', () => {
    document.body.innerHTML = '<input id="category" value="electronics" />';
    const options = [{ id: 'cat-1', name: 'Electronics' }];
    const pick = creatablePickFromDom('category', { name: '' }, options);
    expect(pick).toEqual({ id: 'cat-1', name: 'Electronics' });
  });
});
