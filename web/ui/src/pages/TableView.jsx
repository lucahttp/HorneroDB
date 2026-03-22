import { useState, useEffect, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import axios from 'axios'
import { Xmark } from 'iconoir-react'
import { API_URL } from '../constants'
import { useAuth } from '../context/AuthContext'
import { Button, Badge } from '../components/index.jsx'
import { DataTable } from '../components/DataTable.jsx'
import { CsvImportWizard } from '../components/CsvImportWizard.jsx'
import { notify } from '../components/Toast.jsx'
import TopNavbar from '../components/TopNavbar.jsx'

export default function TableView() {
  const { user, logout } = useAuth()
  const { workspaceId, tableId } = useParams()
  const { t } = useTranslation()
  const navigate = useNavigate()

  const [table, setTable] = useState(null)
  const [columns, setColumns] = useState([])
  const [records, setRecords] = useState([])
  const [roles, setRoles] = useState([])
  const [loading, setLoading] = useState(true)
  const [activeTab, setActiveTab] = useState('data')
  const [tables, setTables] = useState([])

  // CSV Import Wizard ref
  const csvWizardRef = useRef(null)

  const loadData = async () => {
    try {
      const [tableRes, columnsRes, rolesRes, allTablesRes] = await Promise.all([
        axios.get(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}`),
        axios.get(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}/columns`),
        axios.get(`${API_URL}/workspaces/${workspaceId}/roles`),
        axios.get(`${API_URL}/workspaces/${workspaceId}/tables`)
      ])
      const tableData = tableRes.data.data
      const colData = columnsRes.data.data
      setTable(tableData)
      setColumns(colData)
      setRoles(rolesRes.data.data)
      setTables(allTablesRes.data.data || [])

      const relationsToExpand = colData
        .filter(c => c.field_type === 'relation')
        .map(c => c.slug)
        .join(',')

      const expandParam = relationsToExpand ? `?expand=${relationsToExpand}` : ''
      const recordsRes = await axios.get(`${API_URL}/workspaces/${workspaceId}/data/${tableData.slug}${expandParam}`)
      setRecords(recordsRes.data.data || [])
    } catch (err) {
      console.error(err)
    }
    setLoading(false)
  }

  useEffect(() => {
    loadData()
  }, [workspaceId, tableId])


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
      notify(t('error_export_schema') || 'Error exporting schema', 'error')
    }
  }

  const handleCreateRecord = async (data) => {
    await axios.post(`${API_URL}/workspaces/${workspaceId}/data/${table.slug}`, data)
    loadData()
  }

  const deleteRecord = async (id) => {
    if (!confirm(t('confirm_delete_record'))) return
    try {
      await axios.delete(`${API_URL}/workspaces/${workspaceId}/data/${table.slug}/${id}`)
      setRecords(prev => prev.filter(r => r.id !== id))
      notify(t('record_deleted'), 'success')
    } catch (err) {
      console.error(err)
      notify(t('error_delete_record'), 'error')
    }
  }

  const handleUpdateRecord = async (recordId, data) => {
    try {
      await axios.put(`${API_URL}/workspaces/${workspaceId}/data/${table.slug}/${recordId}`, data)
      loadData()
    } catch (err) {
      console.error(err)
      notify(t('error_update_record'), 'error')
    }
  }

  const handleBulkDelete = async (ids) => {
    if (!confirm(t('confirm_delete_records', { count: ids.length }))) return
    try {
      await Promise.all(
        ids.map(id => axios.delete(`${API_URL}/workspaces/${workspaceId}/data/${table.slug}/${id}`))
      )
      notify(t('records_deleted'), 'success')
      loadData()
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
      loadData()
      notify(t('column_created'), 'success')
    } catch (err) {
      console.error(err)
      notify(t('error_create_column'), 'error')
    }
  }

  const handleDeleteColumn = async (columnId) => {
    try {
      await axios.delete(`${API_URL}/workspaces/${workspaceId}/tables/${tableId}/columns/${columnId}`)
      loadData()
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
      loadData()
      notify(t('column_renamed'), 'success')
    } catch (err) {
      console.error(err)
      notify(t('error_rename_column'), 'error')
    }
  }

  const handleOpenCSVImport = () => { csvWizardRef.current?.open() }

  const handleExportCSV = () => {
    if (!records.length || !columns.length) return
    const headers = columns.map(c => c.slug)
    const rows = records.map(r => headers.map(h => {
      const v = r[h] ?? ''
      const s = String(v)
      return s.includes(',') || s.includes('"') || s.includes('\n') ? `"${s.replace(/"/g, '""')}"` : s
    }).join(','))
    const csv = [headers.join(','), ...rows].join('\n')
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${table?.slug || 'export'}_${new Date().toISOString().slice(0, 10)}.csv`
    a.click()
    URL.revokeObjectURL(url)
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
              <Button size="sm" variant="secondary" onClick={handleExportSchema} style={{ gap: '0.5rem', display: 'flex', alignItems: 'center' }}>
                <svg width="1.25rem" height="1.25rem" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" /></svg>
                {t('export_schema') || 'Export Schema'}
              </Button>

              <Button size="sm" variant="secondary" onClick={handleOpenCSVImport} style={{ gap: '0.5rem', display: 'flex', alignItems: 'center' }}>
                <svg width="1.25rem" height="1.25rem" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l4-4m0 0l4 4m-4-4v12" /></svg>
                Import CSV
              </Button>
              <Button size="sm" variant="secondary" onClick={handleExportCSV} style={{ gap: '0.5rem', display: 'flex', alignItems: 'center' }}>
                <svg width="1.25rem" height="1.25rem" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>
                Export CSV
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
            />
          )}

          {activeTab === 'columns' && (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: '1rem' }}>
              {columns.map(col => (
                <div key={col.id} className="card">
                  <div style={{ fontWeight: 700, marginBottom: '0.25rem' }}>{col.name}</div>
                  <div style={{ fontSize: '0.8125rem', color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', marginBottom: '0.75rem' }}>
                    @{col.slug}
                  </div>
                  <Badge variant="primary">{col.field_type}</Badge>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      <CsvImportWizard
        ref={csvWizardRef}
        workspaceId={workspaceId}
        table={table}
        columns={columns}
        onImportComplete={loadData}
      />
    </div>
  )
}
