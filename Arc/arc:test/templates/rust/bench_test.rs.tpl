use criterion::{black_box, criterion_group, criterion_main, Criterion};
use {{CRATE_NAME}}::{{FUNCTION_NAME}};

fn bench_{{FUNCTION_NAME}}(c: &mut Criterion) {
    c.bench_function("{{FUNCTION_NAME}}", |b| {
        b.iter(|| {{FUNCTION_NAME}}(black_box({{SAMPLE_INPUT}})))
    });
}

criterion_group!(benches, bench_{{FUNCTION_NAME}});
criterion_main!(benches);
