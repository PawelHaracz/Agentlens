import { useState, useCallback } from 'react'
import { validateAgentCard, createAgentFromCard } from '../api'
import type { ValidationResult } from '../types'
import CardPreview from './CardPreview'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Card } from '@/components/ui/card'
import { Plus, Upload, AlertCircle, AlertTriangle, CheckCircle } from 'lucide-react'

type Step = 'input' | 'validation' | 'preview' | 'confirm'

interface RegisterAgentDialogProps {
  onRegistered: () => void
}

export default function RegisterAgentDialog({ onRegistered }: RegisterAgentDialogProps) {
  const [open, setOpen] = useState(false)
  const [step, setStep] = useState<Step>('input')
  const [cardJson, setCardJson] = useState('')
  const [jsonError, setJsonError] = useState<string | null>(null)
  const [validationResult, setValidationResult] = useState<ValidationResult | null>(null)
  const [validating, setValidating] = useState(false)
  const [registering, setRegistering] = useState(false)
  const [registerError, setRegisterError] = useState<string | null>(null)

  const reset = useCallback(() => {
    setStep('input')
    setCardJson('')
    setJsonError(null)
    setValidationResult(null)
    setValidating(false)
    setRegistering(false)
    setRegisterError(null)
  }, [])

  const handleOpenChange = (v: boolean) => {
    setOpen(v)
    if (!v) reset()
  }

  const handleJsonChange = (value: string) => {
    setCardJson(value)
    setJsonError(null)
    if (value.trim()) {
      try {
        JSON.parse(value)
      } catch {
        setJsonError('Invalid JSON syntax')
      }
    }
  }

  const handleFileUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = () => {
      const text = reader.result as string
      handleJsonChange(text)
    }
    reader.readAsText(file)
  }

  const handleValidate = async () => {
    if (jsonError || !cardJson.trim()) return
    setValidating(true)
    try {
      const result = await validateAgentCard(cardJson)
      setValidationResult(result)
      if (result.valid && result.errors.length === 0 && result.warnings.length === 0) {
        setStep('preview')
      } else {
        setStep('validation')
      }
    } catch (e) {
      setValidationResult({
        valid: false,
        spec_version: '',
        errors: [{ field: 'request', message: e instanceof Error ? e.message : 'Validation failed' }],
        warnings: [],
      })
      setStep('validation')
    } finally {
      setValidating(false)
    }
  }

  const handleRegister = async () => {
    setRegistering(true)
    setRegisterError(null)
    try {
      await createAgentFromCard(cardJson)
      setOpen(false)
      reset()
      onRegistered()
    } catch (e) {
      setRegisterError(e instanceof Error ? e.message : 'Registration failed')
    } finally {
      setRegistering(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button size="sm">
          <Plus className="mr-2 h-4 w-4" />
          Register Agent
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Register Agent</DialogTitle>
        </DialogHeader>

        {step === 'input' && (
          <div className="space-y-4">
            <Tabs defaultValue="paste">
              <TabsList>
                <TabsTrigger value="paste">Paste JSON</TabsTrigger>
                <TabsTrigger value="upload">Upload File</TabsTrigger>
              </TabsList>
              <TabsContent value="paste">
                <textarea
                  className="w-full h-64 font-mono text-sm border rounded-md p-3 bg-muted/50 focus:outline-none focus:ring-2 focus:ring-ring"
                  placeholder={'{\n  "name": "My Agent",\n  "description": "...",\n  "version": "1.0.0",\n  "supportedInterfaces": [...],\n  "skills": [...]\n}'}
                  value={cardJson}
                  onChange={e => handleJsonChange(e.target.value)}
                />
              </TabsContent>
              <TabsContent value="upload">
                <div className="border-2 border-dashed rounded-md p-8 text-center">
                  <Upload className="mx-auto h-8 w-8 text-muted-foreground mb-2" />
                  <p className="text-sm text-muted-foreground mb-2">Drop a .json file or click to browse</p>
                  <input
                    type="file"
                    accept=".json"
                    onChange={handleFileUpload}
                    className="text-sm"
                  />
                </div>
              </TabsContent>
            </Tabs>

            {jsonError && (
              <p className="text-sm text-destructive flex items-center gap-1">
                <AlertCircle className="h-4 w-4" />
                {jsonError}
              </p>
            )}

            <div className="flex justify-end">
              <Button onClick={handleValidate} disabled={!cardJson.trim() || !!jsonError || validating}>
                {validating ? 'Validating...' : 'Validate'}
              </Button>
            </div>
          </div>
        )}

        {step === 'validation' && validationResult && (
          <div className="space-y-4">
            {validationResult.errors.length > 0 && (
              <Card className="border-destructive bg-destructive/10 p-4">
                <p className="font-medium text-destructive flex items-center gap-1 mb-2">
                  <AlertCircle className="h-4 w-4" />
                  Validation Errors
                </p>
                <ul className="text-sm text-destructive space-y-1">
                  {validationResult.errors.map((err, i) => (
                    <li key={i}><code className="font-mono">{err.field}</code>: {err.message}</li>
                  ))}
                </ul>
              </Card>
            )}

            {validationResult.warnings.length > 0 && (
              <Card className="border-yellow-500 bg-yellow-50 dark:bg-yellow-900/10 p-4">
                <p className="font-medium text-yellow-700 dark:text-yellow-400 flex items-center gap-1 mb-2">
                  <AlertTriangle className="h-4 w-4" />
                  Warnings
                </p>
                <ul className="text-sm text-yellow-700 dark:text-yellow-400 space-y-1">
                  {validationResult.warnings.map((w, i) => (
                    <li key={i}>{w}</li>
                  ))}
                </ul>
              </Card>
            )}

            <div className="flex justify-between">
              <Button variant="outline" onClick={() => setStep('input')}>Back to Edit</Button>
              {validationResult.valid && (
                <Button onClick={() => setStep('preview')}>Continue to Preview</Button>
              )}
            </div>
          </div>
        )}

        {step === 'preview' && validationResult?.preview && (
          <div className="space-y-4">
            <Card className="border-green-500 bg-green-50 dark:bg-green-900/10 p-4">
              <p className="text-sm text-green-700 dark:text-green-400 flex items-center gap-1">
                <CheckCircle className="h-4 w-4" />
                Card validated successfully
              </p>
            </Card>

            <CardPreview preview={validationResult.preview} />

            {registerError && (
              <Card className="border-destructive bg-destructive/10 p-4 text-destructive text-sm">
                {registerError}
              </Card>
            )}

            <div className="flex justify-between">
              <Button variant="outline" onClick={() => setStep('input')}>Back to Edit</Button>
              <Button onClick={handleRegister} disabled={registering}>
                {registering ? 'Registering...' : 'Register Agent'}
              </Button>
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
