# Benchmark Comparison

Compare performance of multiple models side-by-side to find the best model for your use case.

## Quick Start

```bash
# Start the server
offgrid serve

# Compare two models
offgrid compare tinyllama-1.1b phi-2

# Compare with custom settings
offgrid compare llama-3.2-3b mistral-7b phi-2 --iterations 5 --prompt "Explain quantum computing"
```

## Usage

```
offgrid benchmark-compare <model1> <model2> [model3...] [OPTIONS]
offgrid compare <model1> <model2> [model3...] [OPTIONS]
```

### Options

- `--iterations N` - Number of test iterations per model (default: 3)
- `--prompt "text"` - Custom prompt for comparison (default: creative writing task)

### Requirements

- Server must be running (`offgrid serve`)
- At least 2 models must be specified
- Models must be installed (`offgrid list` to see available models)

## Output

The comparison displays a table with:

- **★** - Star symbol marks the fastest model
- **Speed** - Average tokens per second
- **Latency** - Average response time
- **Range** - Min-max tokens/sec across iterations
- **Size** - Model file size and quantization

Speed is color-coded:
- **Green** - Top performance (100%)
- **Yellow** - Good performance (80-100%)
- **Red** - Lower performance (<80%)

Results are sorted by average speed (fastest first).

## Example Output

```
┌─ Benchmark Comparison
│  Comparing 2 models with 3 iterations each

├─ Testing: tinyllama-1.1b-chat-v1.0.Q4_K_M
│  [1/3] ✓ 45.2 tok/s
│  [2/3] ✓ 47.1 tok/s
│  [3/3] ✓ 46.8 tok/s
│  Average: 46.4 tok/s
│
├─ Testing: Llama-3.2-3B-Instruct-Q4_K_M
│  [1/3] ✓ 28.3 tok/s
│  [2/3] ✓ 29.1 tok/s
│  [3/3] ✓ 28.7 tok/s
│  Average: 28.7 tok/s
│
└─ Comparison Results

   Model                                Speed       Latency       Range      Size
   ──────────────────────────────────────────────────────────────────────────────
   ★ tinyllama-1.1b-chat-v1.0.Q4_K_M      46.4 t/s      2.15s    45- 47  637.8 MB Q4_K_M
     Llama-3.2-3B-Instruct-Q4_K_M         28.7 t/s      3.48s    28- 29  1.9 GB Q4_K_M

   ★ = Fastest model
```

## Use Cases

### Speed vs Quality Trade-off

Compare a small fast model with a larger quality model:

```bash
offgrid compare tinyllama-1.1b llama-3-8b
```

### Quantization Impact

Compare different quantization levels of the same model:

```bash
offgrid compare llama-2-7b-q4 llama-2-7b-q5 llama-2-7b-q8
```

### Task-Specific Testing

Test models with your specific use case prompt:

```bash
offgrid compare phi-2 mistral-7b \
  --prompt "Summarize the following article: ..." \
  --iterations 10
```

### Memory Constraints

Find the largest model that fits your RAM:

```bash
offgrid compare tinyllama phi-2 llama-3.2-3b mistral-7b
```

## Tips

1. **Iterations**: Use more iterations (5-10) for more stable averages
2. **Prompt**: Use a representative prompt for your actual use case
3. **Warm-up**: First run may be slower due to model loading
4. **Consistency**: Run comparisons when system is idle for accurate results
5. **Range**: Check min-max range to see performance stability

## Technical Details

- Uses `/v1/completions` API endpoint
- Each model is tested with identical settings
- Metrics collected: tokens/sec, latency, token counts
- Sorting: By average tokens/sec (descending)
- Error handling: Models that fail show "FAILED" status
- Max tokens per test: 100 (adjustable in source)
- Temperature: 0.7 (adjustable in source)

## Related Commands

- `offgrid benchmark <model>` - Detailed benchmark of single model
- `offgrid list` - See installed models
- `offgrid serve` - Start API server
- `offgrid run <model>` - Interactive chat to test quality

## Troubleshooting

**"Server not running"**
```bash
# Start server in another terminal
offgrid serve
```

**"Model not found"**
```bash
# Check installed models
offgrid list

# Install model from catalog
offgrid pull phi-2
```

**0 tok/s results**
- Check llama-server backend is running
- For systemd: `sudo systemctl status llama-server`
- Check server logs for errors

**Inconsistent results**
- Close other applications
- Use more iterations (--iterations 10)
- Check CPU/GPU isn't thermal throttling
