import { useState, useEffect, useRef, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import axios from 'axios'
import { API_URL } from '../constants'
import { useAuth } from '../context/AuthContext'
import { Button, Badge } from '../components/index.jsx'
import { DataTable } from '../components/DataTable.jsx'
import { CsvImportWizard } from '../components/CsvImportWizard.jsx'
import { notify } from '../components/toast.js'
import TopNavbar from '../components/TopNavbar.jsx'
import { getEffectivePermissions, isColumnReadable } from '../utils/permissions.js'

export default function TableView() {
  const { user } = useAuth()
  const { workspaceId, tableId } = useParams()
  const { t } = useTranslation()
  const navigate = useNavigate()

  const [table, setTable] = useState(null)
  const [columns, setColumns] = useState([])
  const [records, setRecords] = useState([])
  const [roles, setRoles] = useState([])
  const [loading, setLoading] = useState(true)
  const [recordsLoading, setRecordsLoading] = useState(false)
  const [activeTab, setActiveTab] = useState('data')
  const [tables, setTables] = useState([])

  // ── Pagination ───────────────────────────────────────
  const [page, setPage] = useState(0)
  const [pageSize, setPageSize] = useState(50)
  const [totalRecords, setTotalRecords] = useState(0)

  // ── Server-side search & sorting ─────────────────────
  // `searchInput` is what the user types, `search` is the debounced value that
  // actually reaches the API.
  const [searchInput, setSearchInput] = useState('')
  const [search, setSearch] = useState('')
  const [sortBy, setSortBy] = useState({ column: null, direction: 'asc' })

  // CSV Import Wizard ref
  const csvWizardRef = useRef(null)

  /**
   * Loads the table metadata and the records of the current page.
   * It never touches React state, so it can safely be called from an effect.
   */
  const fetchAll = useCallback(async () => {
    const [tableRes, columnsRes, rolesRes, allTablesRes] = await Promise.all([
      axios.get(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}`),
      axios.get(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}/columns`, { params: { limit: 200 } }),
      // The list endpoints are paginated: ask for enough rows to be sure the
      // role of the current user is part of the answer.
      axios.get(`${API_URL}/workspaces/${workspaceId}/roles`, { params: { limit: 200 } }),
      axios.get(`${API_URL}/workspaces/${workspaceId}/tables`, { params: { limit: 200 } })
    ])

    const tableData = tableRes.data.data
    const colData = columnsRes.data.data || []

    const relationsToExpand = colData
      .filter(c => c.field_type === 'relation')
      .map(c => c.slug)
      .join(',')

    const params = new URLSearchParams()
    params.append('limit', String(pageSize))
    params.append('offset', String(page * pageSize))
    if (relationsToExpand) {
      params.append('expand', relationsToExpand)
    }
    if (search) {
      params.append('search', search)
    }
    if (sortBy.column) {
      params.append('order', `${sortBy.column}:${sortBy.direction}`)
    }

    const recordsRes = await axios.get(
      `${API_URL}/workspaces/${workspaceId}/data/${tableData.slug}?${params.toString()}`
    )

    return {
      table: tableData,
      columns: colData,
      roles: rolesRes.data.data || [],
      tables: allTablesRes.data.data || [],
      records: recordsRes.data.data || [],
      // The API answers with the total amount of rows, not the size of the page.
      total: recordsRes.data.meta?.count ?? recordsRes.data.meta?.total ?? recordsRes.data.data?.length ?? 0,
    }
  }, [workspaceId, tableId, page, pageSize, search, sortBy])

  const applyData = useCallback((data) => {
    setTable(data.table)
    setColumns(data.columns)
    setRoles(data.roles)
    setTables(data.tables)
    setRecords(data.records)
    setTotalRecords(data.total)
  }, [])

  const handleLoadError = useCallback((err) => {
    console.error('Error loading table data:', err)
    if (err.response) {
      console.error('Response status:', err.response.status)
      console.error('Response data:', err.response.data)
      notify(`${t('error_loading_data')}: ${err.response.status}`, 'error')
    } else {
      notify(t('error_loading_data'), 'error')
    }
  }, [t])

  useEffect(() => {
    let cancelled = false

    fetchAll()
      .then(data => {
        if (!cancelled) applyData(data)
      })
      .catch(err => {
        if (!cancelled) handleLoadError(err)
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false)
          setRecordsLoading(false)
        }
      })

    return () => { cancelled = true }
  }, [fetchAll, applyData, handleLoadError])

  /** Reload everything (used after a mutation). */
  const reload = useCallback(() => {
    fetchAll()
      .then(applyData)
      .catch(handleLoadError)
  }, [fetchAll, applyData, handleLoadError])

  // ── Effective permissions of the current role (CLS) ───
  const currentRoleName = user?.role
  const currentRole = roles.find(r => r.name === currentRoleName)
  const effectivePermissions = getEffectivePermissions(
    currentRole?.permissions,
    currentRoleName,
    table?.slug
  )

  const handlePageChange = (nextPage) => {
    setRecordsLoading(true)
    setPage(nextPage)
  }

  const handlePageSizeChange = (nextSize) => {
    setRecordsLoading(true)
    setPageSize(nextSize)
    setPage(0)
  }

  /**
   * Debounces the search box (300 ms) so a burst of keystrokes only produces
   * one request. Any change of the search resets the pagination to the first
   * page, because the filtered result set is a different one.
   */
  useEffect(() => {
    const timer = setTimeout(() => {
      const next = searchInput.trim()
      setSearch(prev => (prev === next ? prev : next))
      setPage(0)
    }, 300)

    return () => clearTimeout(timer)
  }, [searchInput])

  /**
   * Cycles the sort state of a column: none → asc → desc → none.
   */
  const handleSortChange = useCallback((column) => {
    setSortBy(prev => {
      if (prev.column !== column) return { column, direction: 'asc' }
      if (prev.direction === 'asc') return { column, direction: 'desc' }
      return { column: null, direction: 'asc' }
    })
    setPage(0)
  }, [])

  const handleExportSchema = async () => {
    try {
      const res = await axios.get(`${API_URL}/workspaces/${workspaceId}/export`, { responseType: 'blob' })
      const url = window.URL.createObjectURL(new Blob([res.data]))
      const link = document.createElement('a')
      link.href = url
      link.setAttribute('download', `hornerodb_workspace_${workspaceId}.json`)
      document.body.appendChild(link)
      link.click()
      link.parentNode.removeChild(link)
    } catch (err) {
      console.error(err)
      notify(t('error_export_schema'), 'error')
    }
  }

  const handleCreateRecord = async (data) => {
    await axios.post(`${API_URL}/workspaces/${workspaceId}/data/${table.slug}`, data)
    reload()
  }

  const deleteRecord = async (id) => {
    if (!confirm(t('confirm_delete_record'))) return
    try {
      await axios.delete(`${API_URL}/workspaces/${workspaceId}/data/${table.slug}/${id}`)
      setRecords(prev => prev.filter(r => r.id !== id))
      setTotalRecords(prev => Math.max(0, prev - 1))
      notify(t('record_deleted'), 'success')
    } catch (err) {
      console.error(err)
      notify(t('error_delete_record'), 'error')
    }
  }

  const handleUpdateRecord = async (recordId, data) => {
    try {
      await axios.put(`${API_URL}/workspaces/${workspaceId}/data/${table.slug}/${recordId}`, data)
      reload()
    } catch (err) {
      console.error(err)
      notify(t('error_update_record'), 'error')
    }
  }

  const handleBulkDelete = async (ids) => {
    if (!confirm(t('confirm_delete_records', { count: ids.length }))) return
    try {
      // One atomic request instead of N parallel DELETEs: the API either
      // removes every selected record or none of them.
      await axios.delete(`${API_URL}/workspaces/${workspaceId}/data/${table.slug}/bulk`, {
        data: { ids },
      })
      notify(t('records_deleted'), 'success')
      reload()
    } catch (err) {
      console.error(err)
      notify(t('error_delete_records'), 'error')
    }
  }

  const handleCreateColumn = async (name, type, meta = {}) => {
    try {
      await axios.post(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}/columns`, {
        name,
        slug: name.toLowerCase().replace(/\s+/g, '_'),
        field_type: type,
        meta: meta
      })
      reload()
      notify(t('column_created'), 'success')
    } catch (err) {
      console.error(err)
      notify(t('error_create_column'), 'error')
    }
  }

  const handleDeleteColumn = async (columnId) => {
    try {
      await axios.delete(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}/columns/${columnId}`)
      reload()
      notify(t('column_deleted'), 'success')
    } catch (err) {
      console.error(err)
      notify(t('error_delete_column'), 'error')
    }
  }

  const handleRenameColumn = async (columnId, newName) => {
    try {
      await axios.put(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}/columns/${columnId}`, {
        name: newName
      })
      reload()
      notify(t('column_renamed'), 'success')
    } catch (err) {
      console.error(err)
      notify(t('error_rename_column'), 'error')
    }
  }

  const handleOpenCSVImport = () => { csvWizardRef.current?.open() }

  const handleExportCSV = async () => {
    if (!table || !columns.length) return
    try {
      // The grid is paginated, so the export walks through every page.
      const allRecords = []
      const batchSize = 500
      let offset = 0
      let total = Number.POSITIVE_INFINITY

      while (offset < total) {
        const res = await axios.get(`${API_URL}/workspaces/${workspaceId}/data/${table.slug}`, {
          params: { limit: batchSize, offset }
        })
        const batch = res.data.data || []
        allRecords.push(...batch)

        const reported = res.data.meta?.count ?? res.data.meta?.total
        total = typeof reported === 'number' ? reported : allRecords.length

        if (batch.length === 0) break
        offset += batch.length
      }

      const headers = columns.map(c => c.slug)
      const rows = allRecords.map(r => headers.map(h => {
        const v = r[h] ?? ''
        const s = String(v)
        return s.includes(',') || s.includes('"') || s.includes('\n') ? `"${s.replace(/"/g, '""')}"` : s
      }).join(','))
      const csv = [headers.join(','), ...rows].join('\n')
      const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `${table.slug || 'export'}_${new Date().toISOString().slice(0, 10)}.csv`
      a.click()
      URL.revokeObjectURL(url)
    } catch (err) {
      console.error(err)
      notify(t('error_export_csv'), 'error')
    }
  }

  if (loading) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: 'var(--bg)' }}>
        <div className="loading-spinner" />
      </div>
    )
  }

  return (
    <div style={{ minHeight: '100vh', display: 'flex', flexDirection: 'column' }}>
      <TopNavbar workspaceId={workspaceId} />

      <div className="main-content">
        <div className="main-body">
          <button
            onClick={() => navigate(`/workspace/${workspaceId}`)}
            className="btn btn-ghost btn-sm"
            style={{ marginBottom: '1.25rem' }}
          >
            ← {t('back')}
          </button>

          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1.5rem', gap: '0.75rem' }}>
            <h1 style={{ fontSize: '1.75rem', fontWeight: 800, letterSpacing: '-0.02em' }}>{table?.name}</h1>
            <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
              <Button size="sm" variant="secondary" onClick={handleExportSchema} style={{ gap: '0.5rem', display: 'flex', alignItems: 'center', padding: '0.375rem 0.625rem' }}>
                <svg width="1.1rem" height="1.1rem" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" /></svg>
                <span className="hidden sm:inline">{t('export_schema')}</span>
              </Button>

              <Button size="sm" variant="secondary" onClick={handleOpenCSVImport} style={{ gap: '0.5rem', display: 'flex', alignItems: 'center', padding: '0.375rem 0.625rem' }}>
                <svg width="1.1rem" height="1.1rem" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l4-4m0 0l4 4m-4-4v12" /></svg>
                <span className="hidden sm:inline">{t('import_csv')}</span>
              </Button>
              <Button size="sm" variant="secondary" onClick={handleExportCSV} style={{ gap: '0.5rem', display: 'flex', alignItems: 'center', padding: '0.375rem 0.625rem' }}>
                <svg width="1.1rem" height="1.1rem" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>
                <span className="hidden sm:inline">{t('export_csv')}</span>
              </Button>
            </div>
          </div>

          <div className="tabs">
            <button
              className={`tab ${activeTab === 'data' ? 'active' : ''}`}
              onClick={() => setActiveTab('data')}
            >
              📊 {t('data')}
            </button>
            <button
              className={`tab ${activeTab === 'columns' ? 'active' : ''}`}
              onClick={() => setActiveTab('columns')}
            >
              📐 {t('columns')} ({columns.length})
            </button>
          </div>

          {activeTab === 'data' && (
            <>
              <div className="table-toolbar" style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginBottom: '1rem', flexWrap: 'wrap' }}>
                <div style={{ position: 'relative', flex: '1 1 16rem', maxWidth: '22rem' }}>
                  <input
                    type="search"
                    className="form-input"
                    style={{ paddingRight: searchInput ? '2.25rem' : undefined }}
                    placeholder={t('search_records')}
                    aria-label={t('search_records_aria')}
                    value={searchInput}
                    onChange={e => setSearchInput(e.target.value)}
                  />
                  {searchInput && (
                    <button
                      type="button"
                      onClick={() => setSearchInput('')}
                      aria-label={t('clear_search')}
                      title={t('clear_search')}
                      style={{
                        position: 'absolute', right: '0.5rem', top: '50%', transform: 'translateY(-50%)',
                        background: 'transparent', border: 'none', cursor: 'pointer',
                        color: 'var(--text-muted)', fontSize: '1rem', lineHeight: 1, padding: '0.25rem',
                      }}
                    >
                      ✕
                    </button>
                  )}
                </div>
                {sortBy.column && (
                  <span className="badge badge-gray" style={{ whiteSpace: 'nowrap' }}>
                    {t('sort_by')}: {sortBy.column} {sortBy.direction === 'asc' ? '↑' : '↓'}
                  </span>
                )}
              </div>

              {search && records.length === 0 && !recordsLoading && (
                <p style={{ color: 'var(--text-secondary)', fontSize: '0.875rem', marginBottom: '0.75rem' }}>
                  {t('no_records_found')}
                </p>
              )}

              <DataTable
                columns={columns}
                records={records}
                onCreateRecord={handleCreateRecord}
                onDeleteRecord={deleteRecord}
                onUpdateRecord={handleUpdateRecord}
                onBulkDelete={handleBulkDelete}
                onCreateColumn={handleCreateColumn}
                onDeleteColumn={handleDeleteColumn}
                onRenameColumn={handleRenameColumn}
                workspaceId={workspaceId}
                tables={tables}
                permissions={effectivePermissions}
                sortBy={sortBy}
                onSortChange={handleSortChange}
                pagination={{
                  total: totalRecords,
                  page,
                  pageSize,
                  onPageChange: handlePageChange,
                  onPageSizeChange: handlePageSizeChange,
                }}
              />
            </>
          )}

          {activeTab === 'columns' && (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: '1rem' }}>
              {columns.map(col => {
                const readable = isColumnReadable(effectivePermissions.readableColumns, col.slug)
                return (
                  <div key={col.id} className="card">
                    <div style={{ fontWeight: 700, marginBottom: '0.25rem' }}>{col.name}</div>
                    <div style={{ fontSize: '0.8125rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', marginBottom: '0.75rem' }}>
                      @{col.slug}
                    </div>
                    <Badge variant="primary">{col.field_type}</Badge>
                    {!readable && (
                      <Badge variant="error" style={{ marginLeft: '0.5rem' }}>{t('restricted')}</Badge>
                    )}
                  </div>
                )
              })}
            </div>
          )}

          {recordsLoading && (
            <div style={{ display: 'flex', justifyContent: 'center', padding: '1rem 0' }}>
              <div className="loading-spinner" />
            </div>
          )}
        </div>
      </div>

      <CsvImportWizard
        ref={csvWizardRef}
        workspaceId={workspaceId}
        table={table}
        columns={columns}
        onImportComplete={reload}
      />
    </div>
  )
}
